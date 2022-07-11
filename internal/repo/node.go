package repo

import (
	"context"
	"github.com/amimof/blipblop/api/services/nodes/v1"
	"github.com/amimof/blipblop/pkg/cache"
	"log"
	"time"
)

var nodeRepo NodeRepo

type NodeRepo interface {
	Get(ctx context.Context, key string) (*nodes.Node, error)
	List(ctx context.Context) ([]*nodes.Node, error)
	Create(ctx context.Context, node *nodes.Node) error
	Delete(ctx context.Context, key string) error
	Update(ctx context.Context, node *nodes.Node) error
}

type inmemNodeRepo struct {
	cache *cache.Cache
}

func (u *inmemNodeRepo) List(ctx context.Context) ([]*nodes.Node, error) {
	var n []*nodes.Node
	for _, k := range u.cache.ListKeys() {
		node, _ := u.Get(ctx, k)
		n = append(n, node)
	}
	return n, nil
}

func (u *inmemNodeRepo) Get(ctx context.Context, key string) (*nodes.Node, error) {
	val := u.cache.Get(key)
	if val == nil {
		return nil, nil
	}
	return val.Value.(*nodes.Node), nil
}

func (u *inmemNodeRepo) Create(ctx context.Context, node *nodes.Node) error {
	log.Printf("Node %+v", node)
	u.cache.Set(node.Name, node)
	return nil
}

func (u *inmemNodeRepo) Update(ctx context.Context, node *nodes.Node) error {
	u.cache.Set(node.Name, node)
	return nil
}

func (u *inmemNodeRepo) Delete(ctx context.Context, key string) error {
	u.cache.Delete(key)
	return nil
}

func newNodeRepo() NodeRepo {
	c := cache.New()
	c.TTL = time.Hour * 24
	return &inmemNodeRepo{
		cache: c,
	}
}

func NewNodeRepo() NodeRepo {
	if nodeRepo == nil {
		nodeRepo = newNodeRepo()
	}
	return nodeRepo
}
