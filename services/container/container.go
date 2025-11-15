// Package container provides the server implemenetation for the container service
package container

import (
	"context"

	"github.com/amimof/blipblop/api/services/containers/v1"
	"github.com/amimof/blipblop/pkg/events"
	"github.com/amimof/blipblop/pkg/logger"
	"github.com/amimof/blipblop/pkg/repository"
	"google.golang.org/grpc"
)

type NewServiceOption func(s *ContainerService)

func WithLogger(l logger.Logger) NewServiceOption {
	return func(s *ContainerService) {
		s.logger = l
	}
}

func WithExchange(e *events.Exchange) NewServiceOption {
	return func(s *ContainerService) {
		s.exchange = e
	}
}

type ContainerService struct {
	containers.UnimplementedContainerServiceServer
	local    containers.ContainerServiceClient
	logger   logger.Logger
	exchange *events.Exchange
}

func (c *ContainerService) Register(server *grpc.Server) error {
	// server.RegisterService(&containers.ContainerService_ServiceDesc, c)
	containers.RegisterContainerServiceServer(server, c)
	return nil
}

func (c *ContainerService) Get(ctx context.Context, req *containers.GetRequest) (*containers.GetResponse, error) {
	return c.local.Get(ctx, req)
}

func (c *ContainerService) List(ctx context.Context, req *containers.ListRequest) (*containers.ListResponse, error) {
	return c.local.List(ctx, req)
}

func (c *ContainerService) Create(ctx context.Context, req *containers.CreateRequest) (*containers.CreateResponse, error) {
	return c.local.Create(ctx, req)
}

func (c *ContainerService) Delete(ctx context.Context, req *containers.DeleteRequest) (*containers.DeleteResponse, error) {
	return c.local.Delete(ctx, req)
}

func (c *ContainerService) Kill(ctx context.Context, req *containers.KillRequest) (*containers.KillResponse, error) {
	return c.local.Kill(ctx, req)
}

func (c *ContainerService) Start(ctx context.Context, req *containers.StartRequest) (*containers.StartResponse, error) {
	return c.local.Start(ctx, req)
}

func (c *ContainerService) Update(ctx context.Context, req *containers.UpdateRequest) (*containers.UpdateResponse, error) {
	return c.local.Update(ctx, req)
}

func (c *ContainerService) Patch(ctx context.Context, req *containers.PatchRequest) (*containers.PatchResponse, error) {
	return c.local.Patch(ctx, req)
}

func (c *ContainerService) UpdateStatus(ctx context.Context, req *containers.UpdateStatusRequest) (*containers.UpdateStatusResponse, error) {
	return c.local.UpdateStatus(ctx, req)
}

func NewService(repo repository.ContainerRepository, opts ...NewServiceOption) *ContainerService {
	s := &ContainerService{
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
