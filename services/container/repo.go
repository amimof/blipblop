package container

import (
	"context"
	"github.com/amimof/blipblop/api/services/containers/v1"
	"github.com/amimof/blipblop/pkg/cache"
	"github.com/containerd/containerd"
	gocni "github.com/containerd/go-cni"
	"time"
)

var containerRepo Repo

type Repo interface {
	GetAll(ctx context.Context) ([]*containers.Container, error)
	Get(ctx context.Context, key string) (*containers.Container, error)
	Create(ctx context.Context, container *containers.Container) error
	Delete(ctx context.Context, key string) error
	Update(ctx context.Context, container *containers.Container) error
	Start(ctx context.Context, key string) error
	Stop(ctx context.Context, key string) error
	Kill(ctx context.Context, key string) error
}

// containerdRepo is a live containerd environment. Data is stored and fetched in and from the contaienrd runtime
type containerdRepo struct {
	client *containerd.Client
	cni    gocni.CNI
}

type inmemRepo struct {
	cache *cache.Cache
}

func (i *inmemRepo) GetAll(ctx context.Context) ([]*containers.Container, error) {
	var c []*containers.Container
	for _, key := range i.cache.ListKeys() {
		container, _ := i.Get(ctx, key)
		if container != nil {
			c = append(c, container)
		}
	}
	return c, nil
}
func (i *inmemRepo) Get(ctx context.Context, key string) (*containers.Container, error) {
	var c *containers.Container
	item := i.cache.Get(key)
	if item != nil {
		c = item.Value.(*containers.Container)
	}
	return c, nil
}
func (i *inmemRepo) Create(ctx context.Context, container *containers.Container) error {
	i.cache.Set(container.GetName(), container)
	return nil
}
func (i *inmemRepo) Delete(ctx context.Context, key string) error {
	i.cache.Delete(key)
	return nil
}
func (i *inmemRepo) Update(ctx context.Context, container *containers.Container) error {
	i.cache.Set(container.GetName(), container)
	return nil
}

func (i *inmemRepo) Start(ctx context.Context, key string) error {
	return nil
}

func (i *inmemRepo) Stop(ctx context.Context, key string) error {
	return nil
}

func (i *inmemRepo) Kill(ctx context.Context, key string) error {
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
	if containerRepo == nil {
		containerRepo = newInMemRepo()
	}
	return containerRepo
}
