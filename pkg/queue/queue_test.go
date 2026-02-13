package queue

import (
	"context"
	"log"
	"testing"
	"time"

	tasksv1 "github.com/amimof/voiyd/api/services/tasks/v1"
	"github.com/amimof/voiyd/api/types/v1"
	"github.com/amimof/voiyd/pkg/logger"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

var tasks []*tasksv1.Task = []*tasksv1.Task{
	{
		Meta: &types.Meta{
			Name:            "task-1",
			Generation:      1,
			ResourceVersion: 1,
			Uid:             uuid.New().String(),
		},
		Config: &tasksv1.Config{
			Image: "docker.io/nginx/library:latest",
		},
	},
	{
		Meta: &types.Meta{
			Name:            "task-2",
			Generation:      1,
			ResourceVersion: 1,
			Uid:             uuid.New().String(),
		},
		Config: &tasksv1.Config{
			Image: "docker.io/nginx/library:latest",
		},
	},
	{
		Meta: &types.Meta{
			Name:            "task-3",
			Generation:      1,
			ResourceVersion: 1,
			Uid:             uuid.New().String(),
		},
		Config: &tasksv1.Config{
			Image: "docker.io/nginx/library:latest",
		},
	},
	{
		Meta: &types.Meta{
			Name:            "task-4",
			Generation:      1,
			ResourceVersion: 1,
			Uid:             uuid.New().String(),
		},
		Config: &tasksv1.Config{
			Image: "docker.io/nginx/library:latest",
		},
	},
	{
		Meta: &types.Meta{
			Name:            "task-5",
			Generation:      1,
			ResourceVersion: 1,
			Uid:             uuid.New().String(),
		},
		Config: &tasksv1.Config{
			Image: "docker.io/nginx/library:latest",
		},
	},
}

func TestQueueDeduplication(t *testing.T) {
	q := NewTaskQueue(logger.NilLogger{})
	ctx := context.Background()
	for range 5 {
		err := q.Enqueue(ctx, &QueueItem{
			Task:       tasks[0],
			EnqueuedAt: time.Now(),
			Handler: func(ctx context.Context, t *tasksv1.Task) error {
				return nil
			},
		})
		if err != nil {
			log.Fatal(err)
		}
	}

	assert.Len(t, q.items, 1, "expect length of queue to be 1")
}

func TestQueueDequeueRequeue(t *testing.T) {
	q := NewTaskQueue(logger.NilLogger{})

	ctx := context.Background()
	for _, t := range tasks {
		err := q.Enqueue(ctx, &QueueItem{
			Task:       t,
			EnqueuedAt: time.Now(),
			Handler: func(ctx context.Context, t *tasksv1.Task) error {
				return nil
			},
		})
		if err != nil {
			log.Fatal(err)
		}
	}

	assert.Len(t, q.items, 5, "expect length of queue to be 5")

	item, err := q.Dequeue()
	if err != nil {
		t.Fatal(err)
	}

	assert.Len(t, q.items, 4, "expected length of queue to be 4")

	err = q.Requeue(ctx, item)
	if err != nil {
		t.Fatal(err)
	}

	assert.Len(t, q.items, 5, "expected length of queue to be 5")
}
