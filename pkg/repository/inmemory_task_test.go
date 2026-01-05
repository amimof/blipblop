package repository

import (
	"context"
	"testing"

	"github.com/amimof/voiyd/api/types/v1"
	"github.com/amimof/voiyd/pkg/labels"
	"github.com/stretchr/testify/assert"

	tasksv1 "github.com/amimof/voiyd/api/services/tasks/v1"
)

func initInMemTaskRepo(ctx context.Context, repo TaskRepository) (TaskRepository, error) {
	tasks := []*tasksv1.Task{
		{
			Meta: &types.Meta{
				Name: "container-without-labels",
			},
		},
		{
			Meta: &types.Meta{
				Name: "container-with-labels",
				Labels: map[string]string{
					"app": "default",
				},
			},
		},
		{
			Meta: &types.Meta{
				Name: "container-with-multiple-labels",
				Labels: map[string]string{
					"app":                    "backend",
					"region":                 "west",
					"voiyd.io/container-set": "test-set",
				},
			},
		},
	}

	for _, task := range tasks {
		err := repo.Create(ctx, task)
		if err != nil {
			return nil, err
		}
	}

	return repo, nil
}

func TestListTasksWithFilter(t *testing.T) {
	ctx := context.Background()
	filter := labels.New()
	filter.Set("app", "default")

	repo, err := initInMemTaskRepo(ctx, NewTaskInMemRepo())
	assert.NoError(t, err)

	tasks, err := repo.List(ctx, filter)
	if err != nil {
		t.Fatal(err)
	}

	expected := "container-with-labels"

	assert.Len(t, tasks, 1, "length of containers should be 1")

	for _, task := range tasks {
		assert.Equal(t, task.GetMeta().GetName(), expected, "task should match")
	}
}

func TestListTasksWithNoFilter(t *testing.T) {
	ctx := context.Background()

	repo, err := initInMemTaskRepo(ctx, NewTaskInMemRepo())
	assert.NoError(t, err)

	tasks, err := repo.List(ctx)
	if err != nil {
		t.Fatal(err)
	}

	assert.Len(t, tasks, 3, "length of tasks should be 3")
}
