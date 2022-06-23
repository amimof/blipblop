package repo

import (
	"context"
	"github.com/amimof/blipblop/internal/models"
	"github.com/amimof/blipblop/pkg/cache"
	"github.com/containerd/containerd"
	gocni "github.com/containerd/go-cni"
	"time"
)

type ContainerRepo interface {
	GetAll(ctx context.Context) ([]*models.Container, error)
	Get(ctx context.Context, key string) (*models.Container, error)
	Set(ctx context.Context, unit *models.Container) error
	Delete(ctx context.Context, key string) error
	Start(ctx context.Context, key string) error
	Stop(ctx context.Context, key string) error
	Kill(ctx context.Context, key string) error
}

// containerdRepo is a live containerd environment. Data is stored and fetched in and from the contaienrd runtime
type containerdRepo struct {
	client *containerd.Client
	cni    gocni.CNI
}

type inmemContainerRepo struct {
	cache *cache.Cache
}

func (i *inmemContainerRepo) GetAll(ctx context.Context) ([]*models.Container, error) {
	return nil, nil
}
func (i *inmemContainerRepo) Get(ctx context.Context, key string) (*models.Container, error) {
	return nil, nil
}
func (i *inmemContainerRepo) Set(ctx context.Context, unit *models.Container) error {
	return nil
}
func (i *inmemContainerRepo) Delete(ctx context.Context, key string) error {
	return nil
}
func (i *inmemContainerRepo) Start(ctx context.Context, key string) error {
	return nil
}
func (i *inmemContainerRepo) Stop(ctx context.Context, key string) error {
	return nil
}
func (i *inmemContainerRepo) Kill(ctx context.Context, key string) error {
	return nil
}

func NewInMemContainerRepo() ContainerRepo {
	c := cache.New()
	c.TTL = time.Hour * 24
	return &inmemContainerRepo{
		cache: c,
	}
}
