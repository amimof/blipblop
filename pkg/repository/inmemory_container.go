package repository

import (
	"context"
	"time"

	"github.com/amimof/voiyd/api/services/containers/v1"
	"github.com/amimof/voiyd/pkg/cache"
	"github.com/amimof/voiyd/pkg/labels"
	"github.com/amimof/voiyd/pkg/util"
)

type containerInMemRepo struct {
	cache *cache.Cache
}

func NewContainerInMemRepo() ContainerRepository {
	c := cache.New()
	c.TTL = time.Hour * 24
	return &containerInMemRepo{
		cache: c,
	}
}

func (i *containerInMemRepo) List(ctx context.Context, l ...labels.Label) ([]*containers.Container, error) {
	filter := util.MergeLabels(l...)
	var c []*containers.Container
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

func (i *containerInMemRepo) Get(ctx context.Context, key string) (*containers.Container, error) {
	item := i.cache.Get(key)
	if item == nil {
		return nil, ErrNotFound
	}
	if item.Value == nil {
		return nil, ErrNotFound
	}
	return item.Value.(*containers.Container), nil
}

func (i *containerInMemRepo) Create(ctx context.Context, container *containers.Container) error {
	i.cache.Set(container.GetMeta().GetName(), container)
	return nil
}

func (i *containerInMemRepo) Delete(ctx context.Context, key string) error {
	i.cache.Delete(key)
	return nil
}

func (i *containerInMemRepo) Update(ctx context.Context, container *containers.Container) error {
	i.cache.Set(container.GetMeta().GetName(), container)
	return nil
}
