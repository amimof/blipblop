package repository

import (
	"context"
	"errors"
	"time"

	"github.com/amimof/blipblop/pkg/cache"
)

var ErrNotFound = errors.New("item not found")

type inmemRepo struct {
	cache *cache.Cache
}

func NewInMemRepo() Repository {
	c := cache.New()
	c.TTL = time.Hour * 24
	return &inmemRepo{
		cache: c,
	}
}

func (i *inmemRepo) GetAll(ctx context.Context) ([][]byte, error) {
	var c [][]byte
	for _, key := range i.cache.ListKeys() {
		container, _ := i.Get(ctx, key)
		if container != nil {
			c = append(c, container)
		}
	}
	return c, nil
}

func (i *inmemRepo) Get(ctx context.Context, key string) ([]byte, error) {
	data := i.cache.Get(key)
	if data == nil {
		return nil, ErrNotFound
	}
	return data.Value, nil
}

func (i *inmemRepo) Create(ctx context.Context, id string, data []byte) error {
	i.cache.Set(id, data)
	return nil
}

func (i *inmemRepo) Delete(ctx context.Context, key string) error {
	i.cache.Delete(key)
	return nil
}

func (i *inmemRepo) Update(ctx context.Context, key string, data []byte) error {
	i.cache.Set(key, data)
	return nil
}
