package repository

import (
	"context"
	"time"

	"github.com/amimof/voiyd/pkg/cache"
	"github.com/amimof/voiyd/pkg/labels"
	"github.com/amimof/voiyd/pkg/util"

	tasksv1 "github.com/amimof/voiyd/api/services/tasks/v1"
)

type containerInMemRepo struct {
	cache *cache.Cache
}

func NewTaskInMemRepo() TaskRepository {
	c := cache.New()
	c.TTL = time.Hour * 24
	return &containerInMemRepo{
		cache: c,
	}
}

func (i *containerInMemRepo) List(ctx context.Context, l ...labels.Label) ([]*tasksv1.Task, error) {
	filter := util.MergeLabels(l...)
	var c []*tasksv1.Task
	for _, key := range i.cache.ListKeys() {
		container, _ := i.Get(ctx, key)
		if container != nil {
			filter := labels.NewCompositeSelectorFromMap(filter)
			if filter.Matches(container.GetMeta().GetLabels()) {
				c = append(c, container)
			}
		}
	}
	return c, nil
}

func (i *containerInMemRepo) Get(ctx context.Context, key string) (*tasksv1.Task, error) {
	item := i.cache.Get(key)
	if item == nil {
		return nil, ErrNotFound
	}
	if item.Value == nil {
		return nil, ErrNotFound
	}
	return item.Value.(*tasksv1.Task), nil
}

func (i *containerInMemRepo) Create(ctx context.Context, container *tasksv1.Task) error {
	i.cache.Set(container.GetMeta().GetName(), container)
	return nil
}

func (i *containerInMemRepo) Delete(ctx context.Context, key string) error {
	i.cache.Delete(key)
	return nil
}

func (i *containerInMemRepo) Update(ctx context.Context, container *tasksv1.Task) error {
	i.cache.Set(container.GetMeta().GetName(), container)
	return nil
}
