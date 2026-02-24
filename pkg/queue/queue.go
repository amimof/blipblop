package queue

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/amimof/voiyd/pkg/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"google.golang.org/protobuf/proto"

	tasksv1 "github.com/amimof/voiyd/api/services/tasks/v1"
)

type TaskQueue struct {
	mu      sync.RWMutex
	cond    sync.Cond
	items   map[string]*QueueItem
	popped  map[string]*QueueItem
	logger  logger.Logger
	closed  bool
	metrics *QueueMetrics
	meter   metric.Meter
}

type QueueItem struct {
	Proto        proto.Message
	ResourceName string
	ResourceID   string
	EnqueuedAt   time.Time
	RetryCount   int
	Handler      func(context.Context, proto.Message) error
	ctx          context.Context
	cancel       func()
}

// QueueMetrics tracks performance and operational metrics for the queue
type QueueMetrics struct {
	CurrentDepth   atomic.Int64
	ItemsEnqueued  atomic.Int64
	ItemsDequeued  atomic.Int64
	ItemsProcessed atomic.Int64
	ItemsFailed    atomic.Int64
	TotalRetries   atomic.Int64
}

func (q *TaskQueue) initMetrics() error {
	if _, err := q.meter.Int64ObservableCounter(
		"voiyd.queue.depth",
		metric.WithDescription("Current number of items waiting in queue"),
		metric.WithUnit("{call}"),
		metric.WithInt64Callback(func(ctx context.Context, io metric.Int64Observer) error {
			io.Observe(q.metrics.CurrentDepth.Load())
			return nil
		}),
	); err != nil {
		return err
	}

	if _, err := q.meter.Int64ObservableCounter(
		"voiyd.queue.items.enqueued",
		metric.WithDescription("Total items ever enqueued"),
		metric.WithUnit("{call}"),
		metric.WithInt64Callback(func(ctx context.Context, io metric.Int64Observer) error {
			io.Observe(q.metrics.ItemsEnqueued.Load())
			return nil
		}),
	); err != nil {
		return err
	}

	if _, err := q.meter.Int64ObservableCounter(
		"voiyd.queue.items.dequeued",
		metric.WithDescription("Total items ever dequeued"),
		metric.WithUnit("{call}"),
		metric.WithInt64Callback(func(ctx context.Context, io metric.Int64Observer) error {
			io.Observe(q.metrics.ItemsDequeued.Load())
			return nil
		}),
	); err != nil {
		return err
	}

	if _, err := q.meter.Int64ObservableCounter(
		"voiyd.queue.items.processed",
		metric.WithDescription("Total items successfully processed"),
		metric.WithUnit("{call}"),
		metric.WithInt64Callback(func(ctx context.Context, io metric.Int64Observer) error {
			io.Observe(q.metrics.ItemsProcessed.Load())
			return nil
		}),
	); err != nil {
		return err
	}

	if _, err := q.meter.Int64ObservableCounter(
		"voiyd.queue.items.failed",
		metric.WithDescription("Total items that exceeded max retried"),
		metric.WithUnit("{call}"),
		metric.WithInt64Callback(func(ctx context.Context, io metric.Int64Observer) error {
			io.Observe(q.metrics.ItemsFailed.Load())
			return nil
		}),
	); err != nil {
		return err
	}

	if _, err := q.meter.Int64ObservableCounter(
		"voiyd.queue.retries",
		metric.WithDescription("Total retry attempts"),
		metric.WithUnit("{call}"),
		metric.WithInt64Callback(func(ctx context.Context, io metric.Int64Observer) error {
			io.Observe(q.metrics.TotalRetries.Load())
			return nil
		}),
	); err != nil {
		return err
	}

	return nil
}

// Enqueue adds a task to the queue
func (q *TaskQueue) Enqueue(ctx context.Context, item *QueueItem) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Validate task
	if item.ResourceID == "" || item.ResourceName == "" {
		return fmt.Errorf("ResourceID and ResourceName must not be empty")
	}

	if q.closed {
		return fmt.Errorf("queue is closed, cannot enqueue task %s", item.ResourceName)
	}

	// Set enqueue time if not already set
	if item.EnqueuedAt.IsZero() {
		item.EnqueuedAt = time.Now()
	}

	// Cancel and remove any popped items
	// if item.Task.GetMeta().GetUid() == v.Task.GetMeta().GetUid() {
	if v, ok := q.popped[item.ResourceID]; ok {
		v.cancel()
		delete(q.popped, v.ResourceID)
	}

	q.metrics.ItemsEnqueued.Add(1)
	q.metrics.CurrentDepth.Add(1)
	item.ctx, item.cancel = context.WithCancel(ctx)
	q.items[item.ResourceID] = item

	q.logger.Debug("task enqueued",
		"task", item.ResourceName,
		"retry_count", item.RetryCount,
		"queue_depth", q.metrics.CurrentDepth.Load())

	q.cond.Signal()
	return nil
}

func (q *TaskQueue) Done(t *tasksv1.Task) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if _, ok := q.items[t.GetMeta().GetUid()]; ok {
		q.items[t.GetMeta().GetUid()].cancel()
		delete(q.items, t.GetMeta().GetUid())
	}
}

// Dequeue retrieves and removes a task from the queue
// Blocks until an item is available or queue is closed
func (q *TaskQueue) Dequeue() (*QueueItem, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Wait in a loop to handle spurious wakeups
	for q.Len() == 0 && !q.closed {
		q.cond.Wait()
	}

	// If queue is closed and empty, return error
	if q.closed && q.Len() == 0 {
		return nil, fmt.Errorf("queue is closed and empty")
	}

	item := q.Pop().(*QueueItem)
	if item == nil {
		return nil, fmt.Errorf("queue returned nil item")
	}
	q.popped[item.ResourceID] = item

	waitTime := time.Since(item.EnqueuedAt)

	q.logger.Debug("task dequeued",
		"task", item.ResourceName,
		"wait_time", waitTime,
		"retry_count", item.RetryCount,
		"queue_depth", q.metrics.CurrentDepth.Load())

	q.metrics.ItemsDequeued.Add(1)
	q.metrics.CurrentDepth.Add(-1)

	return item, nil
}

func (q *TaskQueue) Pop() any {
	var n *QueueItem
	for k, v := range q.items {
		n = v
		delete(q.items, k)
		break
	}
	return n
}

// Requeue adds a task back to the queue with incremented retry count
// Used when task processing fails and should be retried
func (q *TaskQueue) Requeue(ctx context.Context, item *QueueItem) error {
	if item == nil {
		return fmt.Errorf("cannot requeue nil item")
	}

	if err := item.ctx.Err(); err != nil {
		return fmt.Errorf("cannot requeue cancelled task: %w", err)
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return fmt.Errorf("queue is closed")
	}

	// Increment retry count
	item.RetryCount++
	q.metrics.TotalRetries.Add(1)

	// Reset enqueue time
	item.EnqueuedAt = time.Now()

	q.logger.Info("requeuing task for retry",
		"task", item.ResourceName,
		"retry_count", item.RetryCount)

	q.metrics.ItemsEnqueued.Add(1)
	q.metrics.CurrentDepth.Add(1)

	q.items[item.ResourceID] = item

	q.cond.Signal()
	return nil
}

// Len returns the current number of items in the queue
func (q *TaskQueue) Len() int {
	return len(q.items)
}

func (q *TaskQueue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return
	}

	q.closed = true

	for _, t := range q.items {
		t.cancel()
	}

	// Wake up all waiting workers so they can see the queue is closed
	q.cond.Broadcast()

	q.logger.Info("task queue closed",
		"remaining_items", q.metrics.CurrentDepth.Load(),
		"total_enqueued", q.metrics.ItemsEnqueued.Load(),
		"total_processed", q.metrics.ItemsProcessed.Load(),
		"total_failed", q.metrics.ItemsFailed.Load())
}

type TaskQueueOption func(*TaskQueue)

func WithMeter(m metric.Meter) TaskQueueOption {
	return func(tq *TaskQueue) {
		tq.meter = m
	}
}

func WithQueueLogger(l logger.Logger) TaskQueueOption {
	return func(tq *TaskQueue) {
		tq.logger = l
	}
}

// NewTaskQueue creates a new unbounded task queue
func NewTaskQueue(opts ...TaskQueueOption) *TaskQueue {
	q := &TaskQueue{
		items:   make(map[string]*QueueItem),
		popped:  make(map[string]*QueueItem),
		logger:  logger.ConsoleLogger{},
		closed:  false,
		metrics: &QueueMetrics{},
		meter:   otel.GetMeterProvider().Meter("voiyd_task_queue"),
	}

	q.cond = *sync.NewCond(&q.mu)

	for _, opt := range opts {
		opt(q)
	}

	_ = q.initMetrics()

	return q
}
