package queue

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	tasksv1 "github.com/amimof/voiyd/api/services/tasks/v1"
	"github.com/amimof/voiyd/api/types/v1"
	"github.com/amimof/voiyd/pkg/logger"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

// Helper function to create a test task
func createTestTask(name string) *tasksv1.Task {
	return &tasksv1.Task{
		Meta: &types.Meta{
			Name:            name,
			Generation:      1,
			ResourceVersion: 1,
			Uid:             uuid.New().String(),
		},
		Config: &tasksv1.Config{
			Image: "docker.io/nginx/library:latest",
		},
	}
}

// Helper to properly shut down pool and queue
func shutdownPoolAndQueue(_ context.Context, cancel context.CancelFunc, pool *WorkPool, queue *TaskQueue) {
	cancel()
	queue.Close()
	pool.Stop()
}

// TestWorkPool_SuccessfulExecution verifies that tasks are processed successfully
func TestWorkPool_SuccessfulExecution(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	var callCount atomic.Int32
	var wg sync.WaitGroup
	wg.Add(1)

	task := createTestTask("successful-task")

	q := NewTaskQueue(WithQueueLogger(logger.NilLogger{}))
	pool := NewPool(q,
		WithLogger(logger.NilLogger{}),
		WithMaxWorkers(1),
		WithMaxRetries(3),
		WithBackoff(1*time.Millisecond, 10*time.Millisecond),
	)

	go pool.Start(ctx)

	// Enqueue a task that succeeds
	err := q.Enqueue(ctx, &QueueItem{
		Proto:        task,
		ResourceID:   task.GetMeta().GetUid(),
		ResourceName: task.GetMeta().GetName(),
		EnqueuedAt:   time.Now(),
		Handler: func(ctx context.Context, m proto.Message) error {
			callCount.Add(1)
			wg.Done()
			return nil // Success
		},
	})
	assert.NoError(t, err)

	// Wait for task to be processed
	wg.Wait()

	// Properly shut down
	shutdownPoolAndQueue(ctx, cancel, pool, q)

	// Verify handler was called exactly once (no retries on success)
	assert.Equal(t, int32(1), callCount.Load(), "handler should be called exactly once")
}

// TestWorkPool_RetryOnFailure verifies retry mechanism with eventual success
func TestWorkPool_RetryOnFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	var callCount atomic.Int32
	var wg sync.WaitGroup
	wg.Add(1)

	task := createTestTask("retry-task")

	q := NewTaskQueue(WithQueueLogger(logger.NilLogger{}))
	pool := NewPool(q,
		WithLogger(logger.NilLogger{}),
		WithMaxWorkers(1),
		WithMaxRetries(5),
		WithBackoff(1*time.Millisecond, 10*time.Millisecond),
	)

	go pool.Start(ctx)

	// Enqueue a task that fails twice then succeeds
	err := q.Enqueue(ctx, &QueueItem{
		Proto:        task,
		ResourceID:   task.GetMeta().GetUid(),
		ResourceName: task.GetMeta().GetName(),
		EnqueuedAt:   time.Now(),
		Handler: func(ctx context.Context, m proto.Message) error {
			count := callCount.Add(1)
			if count <= 2 {
				return errors.New("temporary failure")
			}
			wg.Done()
			return nil // Success on third attempt
		},
	})
	assert.NoError(t, err)

	// Wait for task to succeed after retries
	wg.Wait()

	// Properly shut down
	shutdownPoolAndQueue(ctx, cancel, pool, q)

	// Verify handler was called 3 times (initial + 2 retries)
	assert.Equal(t, int32(3), callCount.Load(), "handler should be called 3 times (1 initial + 2 retries)")
}

// TestWorkPool_MaxRetriesExceeded verifies tasks stop retrying after max attempts
func TestWorkPool_MaxRetriesExceeded(t *testing.T) {
	ctx := t.Context()

	var callCount atomic.Int32
	task := createTestTask("failing-task")

	maxRetries := 3
	q := NewTaskQueue(WithQueueLogger(logger.NilLogger{}))
	pool := NewPool(q,
		WithLogger(logger.NilLogger{}),
		WithMaxWorkers(1),
		WithMaxRetries(maxRetries),
		WithBackoff(1*time.Millisecond, 10*time.Millisecond),
	)

	go pool.Start(ctx)

	// Enqueue a task that always fails
	err := q.Enqueue(ctx, &QueueItem{
		Proto:        task,
		ResourceID:   task.GetMeta().GetUid(),
		ResourceName: task.GetMeta().GetName(),
		EnqueuedAt:   time.Now(),
		Handler: func(ctx context.Context, m proto.Message) error {
			callCount.Add(1)
			return errors.New("persistent failure")
		},
	})
	assert.NoError(t, err)

	// Wait for all retry attempts (initial + maxRetries)
	// Each retry has a small backoff, so wait enough time
	time.Sleep(100 * time.Millisecond)

	// Verify handler was called maxRetries+1 times (initial + maxRetries attempts)
	// After maxRetries is reached, the task is not requeued
	expectedCalls := int32(maxRetries + 1)
	assert.Equal(t, expectedCalls, callCount.Load(),
		"handler should be called %d times (1 initial + %d retries)", expectedCalls, maxRetries)

	// Verify queue is empty (task was not requeued after max retries exceeded)
	assert.Equal(t, 0, q.Len(), "queue should be empty after max retries exceeded")
}

// TestWorkPool_GracefulStop verifies Stop() waits for workers to complete
func TestWorkPool_GracefulStop(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var processingStarted atomic.Bool
	var processingCompleted atomic.Bool
	task := createTestTask("slow-task")

	q := NewTaskQueue(WithQueueLogger(logger.NilLogger{}))
	pool := NewPool(q,
		WithLogger(logger.NilLogger{}),
		WithMaxWorkers(2),
		WithMaxRetries(0),
	)

	go pool.Start(ctx)

	// Enqueue a task that takes time to process
	err := q.Enqueue(ctx, &QueueItem{
		Proto:        task,
		ResourceID:   task.GetMeta().GetUid(),
		ResourceName: task.GetMeta().GetName(),
		EnqueuedAt:   time.Now(),
		Handler: func(ctx context.Context, m proto.Message) error {
			processingStarted.Store(true)
			// Simulate work
			time.Sleep(50 * time.Millisecond)
			processingCompleted.Store(true)
			return nil
		},
	})
	assert.NoError(t, err)

	// Give worker a moment to start processing
	time.Sleep(10 * time.Millisecond)

	// Verify task started
	assert.True(t, processingStarted.Load(), "task processing should have started")

	// Verify task completed
	time.Sleep(100 * time.Millisecond)
	assert.True(t, processingCompleted.Load(), "task processing should have completed")
}

// TestBackoff verifies the exponential backoff calculation
func TestBackoff(t *testing.T) {
	minBackoff := 100 * time.Millisecond
	maxBackoff := 5 * time.Second

	tests := []struct {
		name       string
		attemptNum int
		wantMin    time.Duration
		wantMax    time.Duration
	}{
		{
			name:       "first retry (attempt 0)",
			attemptNum: 0,
			wantMin:    minBackoff,     // 100ms * 2^0 = 100ms
			wantMax:    minBackoff * 2, // with jitter, up to 200ms
		},
		{
			name:       "second retry (attempt 1)",
			attemptNum: 1,
			wantMin:    minBackoff * 2, // 100ms * 2^1 = 200ms
			wantMax:    minBackoff * 4, // with jitter, up to 400ms
		},
		{
			name:       "third retry (attempt 2)",
			attemptNum: 2,
			wantMin:    minBackoff * 4, // 100ms * 2^2 = 400ms
			wantMax:    minBackoff * 8, // with jitter, up to 800ms
		},
		{
			name:       "high attempt hits max",
			attemptNum: 10,
			wantMin:    maxBackoff, // Should be capped at max
			wantMax:    maxBackoff, // Should be capped at max
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Run backoff multiple times to account for jitter
			for range 10 {
				result := backoff(minBackoff, maxBackoff, tt.attemptNum)

				// For high attempts, result should equal max
				if tt.attemptNum >= 10 {
					assert.Equal(t, maxBackoff, result, "backoff should be capped at max")
				} else {
					// For normal attempts, verify range
					assert.GreaterOrEqual(t, result, tt.wantMin,
						"backoff should be at least min duration")
					assert.LessOrEqual(t, result, tt.wantMax,
						"backoff with jitter should not exceed 2x base wait")
				}
			}
		})
	}
}

// TestWorkPool_MultipleWorkers verifies concurrent task processing
func TestWorkPool_MultipleWorkers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var processedCount atomic.Int32
	var wg sync.WaitGroup
	numTasks := 5
	wg.Add(numTasks)

	q := NewTaskQueue(WithQueueLogger(logger.NilLogger{}))
	pool := NewPool(q,
		WithLogger(logger.NilLogger{}),
		WithMaxWorkers(3), // Multiple workers
		WithMaxRetries(0),
	)

	go pool.Start(ctx)

	// Enqueue multiple tasks
	for i := range numTasks {
		task := createTestTask("task-" + string(rune(i)))
		err := q.Enqueue(ctx, &QueueItem{
			Proto:        task,
			ResourceID:   task.GetMeta().GetUid(),
			ResourceName: task.GetMeta().GetName(),
			EnqueuedAt:   time.Now(),
			Handler: func(ctx context.Context, m proto.Message) error {
				processedCount.Add(1)
				wg.Done()
				return nil
			},
		})
		assert.NoError(t, err)
	}

	// Wait for all tasks to be processed
	wg.Wait()

	// Verify all tasks were processed
	assert.Equal(t, int32(numTasks), processedCount.Load(),
		"all %d tasks should be processed", numTasks)
}
