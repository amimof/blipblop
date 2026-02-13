package queue

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	tasksv1 "github.com/amimof/voiyd/api/services/tasks/v1"
	"github.com/amimof/voiyd/pkg/logger"
)

func TestWork(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	var callCount atomic.Int32
	var wg sync.WaitGroup

	q := NewTaskQueue(logger.ConsoleLogger{})
	pool := NewPool(q, WithLogger(logger.ConsoleLogger{}), WithMaxWorkers(1), WithMaxRetries(10), WithBackoff(time.Millisecond*100, time.Millisecond*100))

	go pool.Start(ctx)

	for _, test := range tasks {

		wg.Add(1)
		err := q.Enqueue(ctx, &QueueItem{
			Task:       test,
			EnqueuedAt: time.Now(),
			Handler: func(ctx context.Context, t *tasksv1.Task) error {
				fmt.Printf("task %s: error in handler\n", t.GetMeta().GetName())
				callCount.Add(1)
				wg.Done()
				return errors.New("error")
			},
		})
		assert.NoError(t, err)
	}

	wg.Wait()

	cancel()
	q.Close()
	pool.Stop()

	assert.Equal(t, int32(5), callCount.Load(), "handler should be called exactly 5 times")
}
