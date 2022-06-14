package repo

import (
	"context"
	"github.com/amimof/blipblop/internal/models"
	"github.com/amimof/blipblop/pkg/cache"
	"github.com/containerd/containerd"
	gocni "github.com/containerd/go-cni"
	"time"
)

type UnitRepo interface {
	GetAll(ctx context.Context) ([]*models.Unit, error)
	Get(ctx context.Context, key string) (*models.Unit, error)
	Set(ctx context.Context, unit *models.Unit) error
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

type inmemUnitRepo struct {
	cache *cache.Cache
}

func (i *inmemUnitRepo) GetAll(ctx context.Context) ([]*models.Unit, error) {
	return nil, nil
}
func (i *inmemUnitRepo) Get(ctx context.Context, key string) (*models.Unit, error) {
	return nil, nil
}
func (i *inmemUnitRepo) Set(ctx context.Context, unit *models.Unit) error {
	return nil
}
func (i *inmemUnitRepo) Delete(ctx context.Context, key string) error {
	return nil
}
func (i *inmemUnitRepo) Start(ctx context.Context, key string) error {
	return nil
}
func (i *inmemUnitRepo) Stop(ctx context.Context, key string) error {
	return nil
}
func (i *inmemUnitRepo) Kill(ctx context.Context, key string) error {
	return nil
}

func NewInMemUnitRepo() UnitRepo {
	c := cache.New()
	c.TTL = time.Hour * 24
	return &inmemUnitRepo{
		cache: c,
	}
}
