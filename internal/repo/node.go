package repo

import (
	"context"
	"github.com/amimof/blipblop/internal/models"
	"github.com/amimof/blipblop/pkg/cache"
	"time"
)

var nodeRepo NodeRepo

type NodeRepo interface {
	GetAll(ctx context.Context) ([]*models.Node, error)
	Get(ctx context.Context, key string) (*models.Node, error)
	Create(ctx context.Context, node *models.Node) error
	Delete(ctx context.Context, key string) error
}

type inmemNodeRepo struct {
	cache *cache.Cache
}

func (u *inmemNodeRepo) GetAll(ctx context.Context) ([]*models.Node, error) {
	var nodes []*models.Node
	for _, k := range u.cache.ListKeys() {
		node := u.cache.Get(k).Value.(*models.Node)
		nodes = append(nodes, node)
	}
	return nodes, nil
}

func (u *inmemNodeRepo) Get(ctx context.Context, key string) (*models.Node, error) {
	val := u.cache.Get(key)
	if val == nil {
		return nil, nil
	}
	return val.Value.(*models.Node), nil
}

func (u *inmemNodeRepo) Create(ctx context.Context, node *models.Node) error {
	u.cache.Set(*node.Name, node)
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
