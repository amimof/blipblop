package services

import (
	"context"
	"github.com/amimof/blipblop/internal/models"
	"github.com/amimof/blipblop/internal/repo"
	"strings"
)

type UnitService struct {
	repo  repo.UnitRepo
	units []*models.Unit
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
}
