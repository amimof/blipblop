package repository

import (
	"context"
	"time"

	containersetsv1 "github.com/amimof/blipblop/api/services/containersets/v1"
	"github.com/amimof/blipblop/pkg/cache"
)

type containerSetInMemRepo struct {
	cache *cache.Cache
}

func NewContainerSetInMemRepo() ContainerSetRepository {
	c := cache.New()
	c.TTL = time.Hour * 24
	return &containerSetInMemRepo{
		cache: c,
	}
}

func (i *containerSetInMemRepo) List(ctx context.Context) ([]*containersetsv1.ContainerSet, error) {
	var c []*containersetsv1.ContainerSet
	for _, key := range i.cache.ListKeys() {
		container, _ := i.Get(ctx, key)
		if container != nil {
			c = append(c, container)
		}
	}
	return c, nil
}

func (i *containerSetInMemRepo) Get(ctx context.Context, key string) (*containersetsv1.ContainerSet, error) {
	item := i.cache.Get(key)
	if item.Value == nil {
		return nil, ErrNotFound
	}
	return item.Value.(*containersetsv1.ContainerSet), nil
}

func (i *containerSetInMemRepo) Create(ctx context.Context, container *containersetsv1.ContainerSet) error {
	i.cache.Set(container.GetMeta().GetName(), container)
	return nil
}

func (i *containerSetInMemRepo) Delete(ctx context.Context, key string) error {
	i.cache.Delete(key)
	return nil
}

func (i *containerSetInMemRepo) Update(ctx context.Context, container *containersetsv1.ContainerSet) error {
	i.cache.Set(container.GetMeta().GetName(), container)
	return nil
}
