package repository

import (
	"context"
	"time"

	"github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/pkg/cache"
)

type eventInMemRepo struct {
	cache *cache.Cache
}

func NewEventInMemRepo() EventRepository {
	c := cache.New()
	c.TTL = time.Hour * 24
	return &eventInMemRepo{
		cache: c,
	}
}

func (i *eventInMemRepo) List(ctx context.Context) ([]*events.Event, error) {
	var c []*events.Event
	for _, key := range i.cache.ListKeys() {
		container, _ := i.Get(ctx, key)
		if container != nil {
			c = append(c, container)
		}
	}
	return c, nil
}

func (i *eventInMemRepo) Get(ctx context.Context, key string) (*events.Event, error) {
	item := i.cache.Get(key)
	if item.Value == nil {
		return nil, ErrNotFound
	}
	return item.Value.(*events.Event), nil
}

func (i *eventInMemRepo) Create(ctx context.Context, container *events.Event) error {
	i.cache.Set(container.GetId(), container)
	return nil
}

func (i *eventInMemRepo) Delete(ctx context.Context, key string) error {
	i.cache.Delete(key)
	return nil
}

func (i *eventInMemRepo) Update(ctx context.Context, container *events.Event) error {
	i.cache.Set(container.GetId(), container)
	return nil
}
