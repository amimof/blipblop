package services

import (
	"context"
	"github.com/amimof/blipblop/internal/models"
	"github.com/amimof/blipblop/internal/repo"
	"github.com/amimof/blipblop/api/services/containers/v1"
	"strings"
)

var containerService *ContainerService

type ContainerService struct {
	repo  repo.ContainerRepo
	containers.UnimplementedContainerServiceServer
}

func (c *ContainerService) GetTest(ctx context.Context, container *containers.GetContainerRequest) (*containers.GetContainerResponse, error) {
	//return c.repo.Get(ctx, id)
	return &containers.GetContainerResponse{
		Container: &containers.Container{
			Image: "docker.io/amimo/node-cert-exporter:latest",
		},
	}, nil
}

func (c *ContainerService) CreateTest(ctx context.Context, container *containers.CreateContainerRequest) (*containers.CreateContainerResponse, error) {
	//return c.repo.Get(ctx, id)
	return &containers.CreateContainerResponse{
		Container: container.Container,
	}, nil
}

func (c *ContainerService) Get(id string) (*models.Container, error) {
	return c.repo.Get(context.Background(), id)
}

func (c *ContainerService) All() ([]*models.Container, error) {
	return c.repo.GetAll(context.Background())
}

func (c *ContainerService) Create(unit *models.Container) error {
	return c.repo.Set(context.Background(), unit)
}

func (c *ContainerService) Start(id string) error {
	return c.repo.Start(context.Background(), id)
}

func (c *ContainerService) Stop(id string) error {
	err := c.repo.Stop(context.Background(), id)
	if err != nil && !strings.Contains(err.Error(), "process already finished") {
		return err
	}
	return nil
}

func (c *ContainerService) Delete(id string) error {
	return c.repo.Delete(context.Background(), id)
}

func newContainerService(r repo.ContainerRepo) *ContainerService {
	return &ContainerService{
		repo: r,
	}
}

func NewContainerService(r repo.ContainerRepo) *ContainerService {
	if containerService == nil {
		containerService = newContainerService(repo.NewInMemContainerRepo())
	}
	return containerService
}
