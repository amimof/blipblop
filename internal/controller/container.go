package controller

import (
	"context"
	"github.com/amimof/blipblop/internal/models"
	"github.com/amimof/blipblop/internal/repo"
	"strings"
)

var containerController *ContainerController

type ContainerController struct {
	repo  repo.ContainerRepo
}

func (c *ContainerController) Get(id string) (*models.Container, error) {
	return c.repo.Get(context.Background(), id)
}

func (c *ContainerController) All() ([]*models.Container, error) {
	return c.repo.GetAll(context.Background())
}

func (c *ContainerController) Create(unit *models.Container) error {
	return c.repo.Set(context.Background(), unit)
}

func (c *ContainerController) Start(id string) error {
	return c.repo.Start(context.Background(), id)
}

func (c *ContainerController) Stop(id string) error {
	err := c.repo.Stop(context.Background(), id)
	if err != nil && !strings.Contains(err.Error(), "process already finished") {
		return err
	}
	return nil
}

func (c *ContainerController) Delete(id string) error {
	return c.repo.Delete(context.Background(), id)
}

func newContainerController(r repo.ContainerRepo) *ContainerController {
	return &ContainerController{
		repo: r,
	}
}

func NewContainerController(r repo.ContainerRepo) *ContainerController {
	if containerController == nil {
		containerController = newContainerController(repo.NewInMemContainerRepo())
	}
	return containerController
}
