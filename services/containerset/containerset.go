package containerset

import (
	"context"

	containersetsv1 "github.com/amimof/blipblop/api/services/containersets/v1"
	"github.com/amimof/blipblop/pkg/eventsv2"
	"github.com/amimof/blipblop/pkg/logger"
	"github.com/amimof/blipblop/pkg/repository"
	"google.golang.org/grpc"
)

type NewServiceOption func(s *ContainerSetService)

func WithLogger(l logger.Logger) NewServiceOption {
	return func(s *ContainerSetService) {
		s.logger = l
	}
}

func WithExchange(e *eventsv2.Exchange) NewServiceOption {
	return func(s *ContainerSetService) {
		s.exchange = e
	}
}

type ContainerSetService struct {
	containersetsv1.UnimplementedContainerSetServiceServer
	local    containersetsv1.ContainerSetServiceClient
	logger   logger.Logger
	exchange *eventsv2.Exchange
}

func (c *ContainerSetService) Register(server *grpc.Server) error {
	server.RegisterService(&containersetsv1.ContainerSetService_ServiceDesc, c)
	// containers.RegisterContainerServiceServer(server, c)
	return nil
}

func (c *ContainerSetService) Get(ctx context.Context, req *containersetsv1.GetContainerSetRequest) (*containersetsv1.GetContainerSetResponse, error) {
	return c.local.Get(ctx, req)
}

func (c *ContainerSetService) List(ctx context.Context, req *containersetsv1.ListContainerSetRequest) (*containersetsv1.ListContainerSetResponse, error) {
	return c.local.List(ctx, req)
}

func (c *ContainerSetService) Create(ctx context.Context, req *containersetsv1.CreateContainerSetRequest) (*containersetsv1.CreateContainerSetResponse, error) {
	return c.local.Create(ctx, req)
}

func (c *ContainerSetService) Delete(ctx context.Context, req *containersetsv1.DeleteContainerSetRequest) (*containersetsv1.DeleteContainerSetResponse, error) {
	return c.local.Delete(ctx, req)
}

func (c *ContainerSetService) Update(ctx context.Context, req *containersetsv1.UpdateContainerSetRequest) (*containersetsv1.UpdateContainerSetResponse, error) {
	return c.local.Update(ctx, req)
}

func NewService(repo repository.ContainerSetRepository, opts ...NewServiceOption) *ContainerSetService {
	s := &ContainerSetService{
		logger: logger.ConsoleLogger{},
	}

	for _, opt := range opts {
		opt(s)
	}

	s.local = &local{
		repo:     repo,
		exchange: s.exchange,
		logger:   s.logger,
	}

	return s
}
