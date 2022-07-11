package container

import (
	"context"
	"errors"
	"fmt"
	"github.com/amimof/blipblop/api/services/containers/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
	"sync"
	"time"
)

var containerService *ContainerService

type ContainerService struct {
	containers.UnimplementedContainerServiceServer
	repo Repo
	mu   sync.Mutex
}

func (c *ContainerService) Get(ctx context.Context, req *containers.GetContainerRequest) (*containers.GetContainerResponse, error) {
	container, err := c.Repo().Get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	if container == nil {
		return nil, errors.New(fmt.Sprintf("container not found %s", req.GetId()))
	}
	return &containers.GetContainerResponse{
		Container: container,
	}, nil
}

func (c *ContainerService) List(ctx context.Context, req *containers.ListContainerRequest) (*containers.ListContainerResponse, error) {
	ctrs, err := c.Repo().GetAll(ctx)
	if err != nil {
		return nil, err
	}
	return &containers.ListContainerResponse{
		Containers: ctrs,
	}, nil
}

func (c *ContainerService) Create(ctx context.Context, req *containers.CreateContainerRequest) (*containers.CreateContainerResponse, error) {
	container := req.GetContainer()
	container.Created = timestamppb.New(time.Now())
	container.Updated = timestamppb.New(time.Now())
	container.Revision = 1
	err := c.Repo().Create(ctx, container)
	if err != nil {
		return nil, err
	}
	container, err = c.Repo().Get(ctx, container.GetName())
	if err != nil {
		return nil, err
	}
	return &containers.CreateContainerResponse{
		Container: container,
	}, nil
}

func (c *ContainerService) Delete(ctx context.Context, req *containers.DeleteContainerRequest) (*containers.DeleteContainerResponse, error) {
	err := c.Repo().Delete(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &containers.DeleteContainerResponse{
		Id: req.GetId(),
	}, nil
}

func (c *ContainerService) Repo() Repo {
	if c.repo != nil {
		return c.repo
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return NewInMemRepo()
}

func newContainerService(repo Repo) *ContainerService {
	return &ContainerService{
		repo: repo,
	}
}

func NewContainerService() *ContainerService {
	if containerService == nil {
		containerService = newContainerService(NewInMemRepo())
	}
	return containerService
}
