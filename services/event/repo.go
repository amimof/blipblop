package event

import (
	"context"
	"github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/pkg/cache"
	"time"
)

var eventRepo Repo

type Repo interface {
	Get(ctx context.Context, key string) (*events.Event, error)
	List(ctx context.Context) ([]*events.Event, error)
	Create(ctx context.Context, event *events.Event) error
	Delete(ctx context.Context, key string) error
	Update(ctx context.Context, event *events.Event) error
}

type inmemRepo struct {
	cache *cache.Cache
}

func (u *inmemRepo) List(ctx context.Context) ([]*events.Event, error) {
	var n []*events.Event
	for _, k := range u.cache.ListKeys() {
		event, _ := u.Get(ctx, k)
		n = append(n, event)
	}
	return n, nil
}

func (u *inmemRepo) Get(ctx context.Context, key string) (*events.Event, error) {
	val := u.cache.Get(key)
	if val == nil {
		return nil, nil
	}
	return val.Value.(*events.Event), nil
}

func (u *inmemRepo) Create(ctx context.Context, event *events.Event) error {
	u.cache.Set(event.GetId(), event)
	return nil
}

func (u *inmemRepo) Update(ctx context.Context, event *events.Event) error {
	u.cache.Set(event.GetId(), event)
	return nil
}

func (u *inmemRepo) Delete(ctx context.Context, key string) error {
	u.cache.Delete(key)
	return nil
}

func newInMemRepo() Repo {
	c := cache.New()
	c.TTL = time.Hour * 24
	return &inmemRepo{
		cache: c,
	}
}

func NewInMemRepo() Repo {
	if eventRepo == nil {
		eventRepo = newInMemRepo()
	}
	return eventRepo
}
