package container

import (
	"context"

	"github.com/amimof/blipblop/api/services/containers/v1"
	"github.com/amimof/blipblop/pkg/logger"
	"github.com/amimof/blipblop/pkg/repository"
	"github.com/amimof/blipblop/services/event"
	"google.golang.org/grpc"
)

type NewServiceOption func(s *ContainerService)

func WithLogger(l logger.Logger) NewServiceOption {
	return func(s *ContainerService) {
		s.logger = l
	}
}

type ContainerService struct {
	containers.UnimplementedContainerServiceServer
	local  containers.ContainerServiceClient
	logger logger.Logger
}

func (c *ContainerService) Register(server *grpc.Server) error {
	server.RegisterService(&containers.ContainerService_ServiceDesc, c)
	// containers.RegisterContainerServiceServer(server, c)
	return nil
}

func (c *ContainerService) Get(ctx context.Context, req *containers.GetContainerRequest) (*containers.GetContainerResponse, error) {
	return c.local.Get(ctx, req)
}

func (c *ContainerService) List(ctx context.Context, req *containers.ListContainerRequest) (*containers.ListContainerResponse, error) {
	return c.local.List(ctx, req)
}

func (c *ContainerService) Create(ctx context.Context, req *containers.CreateContainerRequest) (*containers.CreateContainerResponse, error) {
	return c.local.Create(ctx, req)
}

func (c *ContainerService) Delete(ctx context.Context, req *containers.DeleteContainerRequest) (*containers.DeleteContainerResponse, error) {
	return c.local.Delete(ctx, req)
}

func (c *ContainerService) Kill(ctx context.Context, req *containers.KillContainerRequest) (*containers.KillContainerResponse, error) {
	return c.local.Kill(ctx, req)
}

func (c *ContainerService) Start(ctx context.Context, req *containers.StartContainerRequest) (*containers.StartContainerResponse, error) {
	return c.local.Start(ctx, req)
}

func (c *ContainerService) Update(ctx context.Context, req *containers.UpdateContainerRequest) (*containers.UpdateContainerResponse, error) {
	return c.local.Update(ctx, req)
}

func NewService(repo repository.ContainerRepository, ev *event.EventService, opts ...NewServiceOption) *ContainerService {
	s := &ContainerService{
		logger: logger.ConsoleLogger{},
	}

	for _, opt := range opts {
		opt(s)
	}

	s.local = &local{
		repo:        repo,
		eventClient: ev,
		logger:      s.logger,
	}

	return s
}
