package repository

import (
	"context"
	"time"

	"github.com/amimof/blipblop/api/services/nodes/v1"
	"github.com/amimof/blipblop/pkg/cache"
)

type nodeInMemRepo struct {
	cache *cache.Cache
}

func NewNodeInMemRepo() NodeRepository {
	c := cache.New()
	c.TTL = time.Hour * 24
	return &nodeInMemRepo{
		cache: c,
	}
}

func (i *nodeInMemRepo) List(ctx context.Context) ([]*nodes.Node, error) {
	var c []*nodes.Node
	for _, key := range i.cache.ListKeys() {
		container, _ := i.Get(ctx, key)
		if container != nil {
			c = append(c, container)
		}
	}
	return c, nil
}

func (i *nodeInMemRepo) Get(ctx context.Context, key string) (*nodes.Node, error) {
	item := i.cache.Get(key)
	if item.Value == nil {
		return nil, ErrNotFound
	}
	return item.Value.(*nodes.Node), nil
}

func (i *nodeInMemRepo) Create(ctx context.Context, container *nodes.Node) error {
	i.cache.Set(container.GetName(), container)
	return nil
}

func (i *nodeInMemRepo) Delete(ctx context.Context, key string) error {
	i.cache.Delete(key)
	return nil
}

func (i *nodeInMemRepo) Update(ctx context.Context, container *nodes.Node) error {
	i.cache.Set(container.GetName(), container)
	return nil
}
