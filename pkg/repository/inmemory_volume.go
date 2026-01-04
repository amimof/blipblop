package repository

import (
	"context"
	"time"

	"github.com/amimof/voiyd/api/services/volumes/v1"
	"github.com/amimof/voiyd/pkg/cache"
	"github.com/amimof/voiyd/pkg/labels"
	"github.com/amimof/voiyd/pkg/util"
)

type volumeInMemRepo struct {
	cache *cache.Cache
}

func NewVolumeInMemRepo() VolumeRepository {
	c := cache.New()
	c.TTL = time.Hour * 24
	return &volumeInMemRepo{
		cache: c,
	}
}

func (i *volumeInMemRepo) List(ctx context.Context, l ...labels.Label) ([]*volumes.Volume, error) {
	filter := util.MergeLabels(l...)
	var c []*volumes.Volume
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

func (i *volumeInMemRepo) Get(ctx context.Context, key string) (*volumes.Volume, error) {
	item := i.cache.Get(key)
	if item == nil {
		return nil, ErrNotFound
	}
	if item.Value == nil {
		return nil, ErrNotFound
	}
	return item.Value.(*volumes.Volume), nil
}

func (i *volumeInMemRepo) Create(ctx context.Context, container *volumes.Volume) error {
	i.cache.Set(container.GetMeta().GetName(), container)
	return nil
}

func (i *volumeInMemRepo) Delete(ctx context.Context, key string) error {
	i.cache.Delete(key)
	return nil
}

func (i *volumeInMemRepo) Update(ctx context.Context, container *volumes.Volume) error {
	i.cache.Set(container.GetMeta().GetName(), container)
	return nil
}
