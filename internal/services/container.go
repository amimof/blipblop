package services

import (
	"context"
	"errors"
	"fmt"
	"github.com/amimof/blipblop/api/services/containers/v1"
	"github.com/amimof/blipblop/internal/repo"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
	"sync"
)

var containerService *ContainerService

type ContainerService struct {
	containers.UnimplementedContainerServiceServer
	repo repo.ContainerRepo
	mu   sync.Mutex
}

func (c *ContainerService) Repo() repo.ContainerRepo {
	if c.repo != nil {
		return c.repo
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return repo.NewInMemContainerRepo()
}

func (c *ContainerService) Get(ctx context.Context, req *containers.GetContainerRequest) (*containers.GetContainerResponse, error) {
	container, err := c.Repo().Get(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	if container == nil {
		return nil, errors.New(fmt.Sprintf("container not found %s", req.Id))
	}
	return &containers.GetContainerResponse{
		Container: &containers.Container{
			Id:     *container.Name,
			Image:  *container.Image,
			Labels: container.Labels,
		},
	}, nil
}

func (c *ContainerService) Create(ctx context.Context, container *containers.CreateContainerRequest) (*containers.CreateContainerResponse, error) {
	return &containers.CreateContainerResponse{
		Container: container.Container,
	}, nil
}

func (c *ContainerService) Delete(ctx context.Context, req *containers.DeleteContainerRequest) (*emptypb.Empty, error) {
	err := c.Repo().Delete(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func newContainerService(repo repo.ContainerRepo) *ContainerService {
	return &ContainerService{
		repo: repo,
	}
}

func NewContainerService() *ContainerService {
	if containerService == nil {
		containerService = newContainerService(repo.NewInMemContainerRepo())
	}
	return containerService
}
