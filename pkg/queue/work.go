package queue

import (
	"context"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/amimof/voiyd/pkg/logger"
)

type WorkPool struct {
	queue *TaskQueue
	// OnTask     func(context.Context, *tasksv1.Task) error
	logger     logger.Logger
	minBackoff time.Duration
	maxBackoff time.Duration
	maxRetries int
	maxWorkers int
	wg         sync.WaitGroup
	stop       chan struct{}
}

func backoff(min, max time.Duration, attemptNum int) time.Duration {
	// Calculate base
	mult := math.Pow(2, float64(attemptNum))
	wait := time.Duration(float64(min) * mult)

	// Add jitter
	jitter := time.Duration(rand.Float64() * float64(wait))
	wait = wait + jitter

	// Cap at max wait duration
	if wait > max {
		wait = max
	}

	return wait
}

func (w *WorkPool) Start(ctx context.Context) {
	for i := 0; i < w.maxWorkers; i++ {
		w.wg.Add(1)
		go w.worker(ctx, i)
	}
}

func (w *WorkPool) worker(ctx context.Context, id int) {
	defer w.wg.Done()

	for {
		select {
		case <-w.stop:
			w.logger.Info("worker stopping", "id", id)
			return
		case <-ctx.Done():
			w.logger.Info("worker context cancelled", "id", id)
			return
		default:
			item, err := w.queue.Dequeue(ctx)
			if err != nil {
				w.logger.Debug("dequeue error, worker exiting", "id", id, "error", err)
				return
			}

			err = item.Handler(ctx, item.Task)
			if err != nil {
				if item.RetryCount >= w.maxRetries {
					// Stop retrying after max attempts
					w.logger.Error("task failed after max retries",
						"task", item.Task.GetMeta().GetName(),
						"retries", item.RetryCount)
					w.queue.metrics.ItemsFailed.Add(1)
					continue // Don't requeue
				}
				// Only apply backoff when requeueing after failure
				wait := backoff(w.minBackoff, w.maxBackoff, item.RetryCount)
				w.logger.Info("task failed, retrying with backoff",
					"task", item.Task.GetMeta().GetName(),
					"retry", item.RetryCount+1,
					"backoff", wait)
				time.Sleep(wait)
				_ = w.queue.Requeue(ctx, item)
			} else {
				// Success case
				w.queue.metrics.ItemsProcessed.Add(1)
				w.logger.Debug("task processed successfully",
					"task", item.Task.GetMeta().GetName())
			}
		}
	}
}

func (w *WorkPool) Stop() {
	close(w.stop) // Signal all workers
	w.wg.Wait()   // Wait for completion
}

type WorkPoolOption func(*WorkPool)

func WithMaxWorkers(n int) WorkPoolOption {
	return func(w *WorkPool) { w.maxWorkers = n }
}

func WithMaxRetries(n int) WorkPoolOption {
	return func(w *WorkPool) { w.maxRetries = n }
}

func WithLogger(l logger.Logger) WorkPoolOption {
	return func(w *WorkPool) { w.logger = l }
}

func WithBackoff(min, max time.Duration) WorkPoolOption {
	return func(w *WorkPool) {
		w.minBackoff = min
		w.maxBackoff = max
	}
}

func NewPool(queue *TaskQueue, opts ...WorkPoolOption) *WorkPool {
	w := &WorkPool{
		queue:      queue,
		maxWorkers: 2,
		maxRetries: 100,
		minBackoff: 5 * time.Second,
		maxBackoff: 60 * time.Second,
		// OnTask:     func(context.Context, *tasksv1.Task) error { return nil },
		logger: logger.ConsoleLogger{},
		stop:   make(chan struct{}),
	}

	for _, opt := range opts {
		opt(w)
	}

	return w
}
