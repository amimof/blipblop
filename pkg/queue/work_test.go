package queue

import (
	"context"
	"errors"
	"fmt"
	"log"
	"testing"
	"testing/synctest"
	"time"

	tasksv1 "github.com/amimof/voiyd/api/services/tasks/v1"
	"github.com/amimof/voiyd/pkg/logger"
)

func TestWork(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		funcCalled := false

		q := NewTaskQueue(logger.NilLogger{})
		pool := NewPool(q, WithLogger(logger.NilLogger{}), WithMaxWorkers(1), WithMaxRetries(10))
		defer pool.Stop()

		go pool.Start(ctx)
		synctest.Wait()

		for _, t := range tasks {

			err := q.Enqueue(ctx, &QueueItem{
				Task:       t,
				EnqueuedAt: time.Now(),
				Handler: func(ctx context.Context, t *tasksv1.Task) error {
					funcCalled = true
					fmt.Printf("task %s: error in handler\n", t.GetMeta().GetName())
					return errors.New("this causes a retry")
				},
			})
			if err != nil {
				log.Fatal(err)
			}
		}

		if funcCalled {
			t.Fatalf("func called before context canceled")
		}

		cancel()
	})
}
