package services

import (
<<<<<<< HEAD
	"strings"
	"context"
	"github.com/containerd/containerd"
	"github.com/amimof/blipblop/internal/models"
	"github.com/amimof/blipblop/internal/repo"
)

type UnitService struct {
	repo repo.UnitRepo
	client *containerd.Client
	units []*models.Unit
=======
	"context"
	"github.com/amimof/blipblop/internal/models"
	"github.com/amimof/blipblop/internal/repo"
	"strings"
)

type UnitService struct {
	repo   repo.UnitRepo
	units  []*models.Unit
>>>>>>> 4483218 (Split server and node:)
}

func (u *UnitService) Get(id string) (*models.Unit, error) {
	return u.repo.Get(context.Background(), id)
}

func (u *UnitService) All() ([]*models.Unit, error) {
	return u.repo.GetAll(context.Background())
}

func (u *UnitService) Create(unit *models.Unit) error {
	return u.repo.Set(context.Background(), unit)
}

func (u *UnitService) Start(id string) error {
	return u.repo.Start(context.Background(), id)
}

func (u *UnitService) Stop(id string) error {
	err := u.repo.Stop(context.Background(), id)
	if err != nil && !strings.Contains(err.Error(), "process already finished") {
		return err
	}
	return nil
}

func (u *UnitService) Delete(id string) error {
	return u.repo.Delete(context.Background(), id)
}

func NewUnitService(repo repo.UnitRepo) *UnitService {
	return &UnitService{
		repo: repo,
	}
<<<<<<< HEAD
}
=======
}
>>>>>>> 4483218 (Split server and node:)
