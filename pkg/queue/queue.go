package queue

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/amimof/voiyd/pkg/events"
	"github.com/amimof/voiyd/pkg/logger"

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
}
type QueueItem struct {
	Task       *tasksv1.Task
	EnqueuedAt time.Time
	RetryCount int
	Handler    events.TaskHandlerFunc
	ctx        context.Context
	cancel     func()
}

// QueueMetrics tracks performance and operational metrics for the queue
type QueueMetrics struct {
	ItemsEnqueued  atomic.Int64
	ItemsDequeued  atomic.Int64
	ItemsProcessed atomic.Int64
	ItemsFailed    atomic.Int64
	TotalRetries   atomic.Int64
	CurrentDepth   atomic.Int64
}

// Enqueue adds a task to the queue
func (q *TaskQueue) Enqueue(ctx context.Context, item *QueueItem) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return fmt.Errorf("queue is closed, cannot enqueue task %s", item.Task.GetMeta().GetName())
	}

	// Validate task
	if item.Task == nil {
		return fmt.Errorf("cannot enqueue nil task")
	}

	// Set enqueue time if not already set
	if item.EnqueuedAt.IsZero() {
		item.EnqueuedAt = time.Now()
	}

	// Cancel and remove any popped items
	// if item.Task.GetMeta().GetUid() == v.Task.GetMeta().GetUid() {
	if v, ok := q.popped[item.Task.GetMeta().GetUid()]; ok {
		v.cancel()
		delete(q.popped, v.Task.GetMeta().GetUid())
	}

	q.metrics.ItemsEnqueued.Add(1)
	q.metrics.CurrentDepth.Add(1)
	item.ctx, item.cancel = context.WithCancel(ctx)
	q.items[item.Task.GetMeta().GetUid()] = item

	q.logger.Debug("task enqueued",
		"task", item.Task.GetMeta().GetName(),
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
	q.popped[item.Task.GetMeta().GetUid()] = item

	waitTime := time.Since(item.EnqueuedAt)

	q.logger.Debug("task dequeued",
		"task", item.Task.GetMeta().GetName(),
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
		"task", item.Task.GetMeta().GetName(),
		"retry_count", item.RetryCount)

	q.metrics.ItemsEnqueued.Add(1)
	q.metrics.CurrentDepth.Add(1)

	q.items[item.Task.GetMeta().GetUid()] = item

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

// NewTaskQueue creates a new unbounded task queue
func NewTaskQueue(logger logger.Logger) *TaskQueue {
	q := &TaskQueue{
		items:   make(map[string]*QueueItem),
		popped:  make(map[string]*QueueItem),
		logger:  logger,
		closed:  false,
		metrics: &QueueMetrics{},
	}

	q.cond = *sync.NewCond(&q.mu)

	return q
}
