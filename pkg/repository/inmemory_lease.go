package repository

import (
	"context"
	"time"

	leasesv1 "github.com/amimof/voiyd/api/services/leases/v1"
	"github.com/amimof/voiyd/pkg/cache"
)

type leaseInMemRepo struct {
	cache *cache.Cache
}

func NewLeaseInMemRepo() LeaseRepository {
	c := cache.New()
	c.TTL = time.Hour * 24
	return &leaseInMemRepo{
		cache: c,
	}
}

func (i *leaseInMemRepo) List(ctx context.Context) ([]*leasesv1.Lease, error) {
	var c []*leasesv1.Lease
	for _, key := range i.cache.ListKeys() {
		container, _ := i.Get(ctx, key)
		if container != nil {
			c = append(c, container)
		}
	}
	return c, nil
}

func (i *leaseInMemRepo) Get(ctx context.Context, key string) (*leasesv1.Lease, error) {
	item := i.cache.Get(key)
	if item.Value == nil {
		return nil, ErrNotFound
	}
	return item.Value.(*leasesv1.Lease), nil
}

func (i *leaseInMemRepo) Create(ctx context.Context, container *leasesv1.Lease) error {
	i.cache.Set(container.GetMeta().GetName(), container)
	return nil
}

func (i *leaseInMemRepo) Delete(ctx context.Context, key string) error {
	i.cache.Delete(key)
	return nil
}

func (i *leaseInMemRepo) Update(ctx context.Context, container *leasesv1.Lease) error {
	i.cache.Set(container.GetMeta().GetName(), container)
	return nil
}
