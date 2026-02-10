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
	items   chan *QueueItem
	logger  logger.Logger
	closed  bool
	metrics *QueueMetrics
}
type QueueItem struct {
	Task       *tasksv1.Task
	EnqueuedAt time.Time
	RetryCount int
	Priority   int // Reserved for future use, always 0 for now
	Handler    events.TaskHandlerFunc
}

// QueueMetrics tracks performance and operational metrics for the queue
type QueueMetrics struct {
	ItemsEnqueued  atomic.Int64 // Total number of items enqueued
	ItemsDequeued  atomic.Int64 // Total number of items dequeued
	ItemsProcessed atomic.Int64 // Total number of items successfully processed
	ItemsFailed    atomic.Int64 // Total number of items that failed after max retries
	TotalRetries   atomic.Int64 // Total number of retry attempts
	CurrentDepth   atomic.Int64 // Current number of items in queue

	// Histogram data for analysis (protected by mutex)
	mu             sync.Mutex
	processingTime []time.Duration // Processing duration samples
	waitTime       []time.Duration // Wait time samples
}

// Enqueue adds a task to the queue
// Returns an error if the queue is closed
func (q *TaskQueue) Enqueue(ctx context.Context, item *QueueItem) error {
	q.mu.RLock()
	defer q.mu.RUnlock()

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

	// Non-blocking send to unbounded queue
	select {
	case q.items <- item:

		q.metrics.ItemsEnqueued.Add(1)
		q.metrics.CurrentDepth.Add(1)

		q.logger.Debug("task enqueued",
			"task", item.Task.GetMeta().GetName(),
			"retry_count", item.RetryCount,
			"queue_depth", q.metrics.CurrentDepth.Load())

		return nil

	case <-ctx.Done():
		return fmt.Errorf("queue context cancelled")
	}
}

// Dequeue retrieves and removes a task from the queue
// Blocks until an item is available or queue is closed
// Returns nil when queue is closed and empty
func (q *TaskQueue) Dequeue(ctx context.Context) (*QueueItem, error) {
	select {
	case item, ok := <-q.items:
		if !ok {
			// Channel closed and drained
			return nil, fmt.Errorf("queue is closed")
		}

		q.metrics.ItemsDequeued.Add(1)
		q.metrics.CurrentDepth.Add(-1)

		// Calculate wait time
		waitTime := time.Since(item.EnqueuedAt)

		q.logger.Debug("task dequeued",
			"task", item.Task.GetMeta().GetName(),
			"wait_time", waitTime,
			"retry_count", item.RetryCount,
			"queue_depth", q.metrics.CurrentDepth.Load())

		return item, nil

	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Requeue adds a task back to the queue with incremented retry count
// Used when task processing fails and should be retried
func (q *TaskQueue) Requeue(ctx context.Context, item *QueueItem) error {
	if item == nil {
		return fmt.Errorf("cannot requeue nil item")
	}

	// Increment retry count
	item.RetryCount++
	q.metrics.TotalRetries.Add(1)

	// Reset enqueue time for new wait measurement
	item.EnqueuedAt = time.Now()

	q.logger.Info("requeuing task for retry",
		"task", item.Task.GetMeta().GetName(),
		"retry_count", item.RetryCount)

	return q.Enqueue(ctx, item)
}

// Len returns the current number of items in the queue
func (q *TaskQueue) Len() int {
	return int(q.metrics.CurrentDepth.Load())
}

func (q *TaskQueue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return
	}

	q.closed = true
	close(q.items)

	q.logger.Info("task queue closed",
		"remaining_items", q.metrics.CurrentDepth.Load(),
		"total_enqueued", q.metrics.ItemsEnqueued.Load(),
		"total_processed", q.metrics.ItemsProcessed.Load(),
		"total_failed", q.metrics.ItemsFailed.Load())
}

// NewTaskQueue creates a new unbounded task queue
func NewTaskQueue(logger logger.Logger) *TaskQueue {
	q := &TaskQueue{
		items:   make(chan *QueueItem, 1000), // Start with buffered channel for performance
		logger:  logger,
		closed:  false,
		metrics: &QueueMetrics{},
	}

	return q
}
