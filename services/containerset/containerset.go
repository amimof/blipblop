package containerset

import (
	"context"

	containersetsv1 "github.com/amimof/blipblop/api/services/containersets/v1"
	"github.com/amimof/blipblop/pkg/events"
	"github.com/amimof/blipblop/pkg/logger"
	"github.com/amimof/blipblop/pkg/repository"
	"google.golang.org/grpc"
)

const Version string = "containerset/v1"

type NewServiceOption func(s *ContainerSetService)

func WithLogger(l logger.Logger) NewServiceOption {
	return func(s *ContainerSetService) {
		s.logger = l
	}
}

func WithExchange(e *events.Exchange) NewServiceOption {
	return func(s *ContainerSetService) {
		s.exchange = e
	}
}

type ContainerSetService struct {
	containersetsv1.UnimplementedContainerSetServiceServer
	local    containersetsv1.ContainerSetServiceClient
	logger   logger.Logger
	exchange *events.Exchange
}

func (c *ContainerSetService) Register(server *grpc.Server) error {
	server.RegisterService(&containersetsv1.ContainerSetService_ServiceDesc, c)
	// containers.RegisterContainerServiceServer(server, c)
	return nil
}

func (c *ContainerSetService) Get(ctx context.Context, req *containersetsv1.GetRequest) (*containersetsv1.GetResponse, error) {
	return c.local.Get(ctx, req)
}

func (c *ContainerSetService) List(ctx context.Context, req *containersetsv1.ListRequest) (*containersetsv1.ListResponse, error) {
	return c.local.List(ctx, req)
}

func (c *ContainerSetService) Create(ctx context.Context, req *containersetsv1.CreateRequest) (*containersetsv1.CreateResponse, error) {
	return c.local.Create(ctx, req)
}

func (c *ContainerSetService) Delete(ctx context.Context, req *containersetsv1.DeleteRequest) (*containersetsv1.DeleteResponse, error) {
	return c.local.Delete(ctx, req)
}

func (c *ContainerSetService) Update(ctx context.Context, req *containersetsv1.UpdateRequest) (*containersetsv1.UpdateResponse, error) {
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
