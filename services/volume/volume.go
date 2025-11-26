package volume

import (
	"context"

	"github.com/amimof/blipblop/api/services/volumes/v1"
	"github.com/amimof/blipblop/pkg/events"
	"github.com/amimof/blipblop/pkg/logger"
	"github.com/amimof/blipblop/pkg/repository"
	"google.golang.org/grpc"
)

type NewServiceOption func(s *VolumeService)

func WithLogger(l logger.Logger) NewServiceOption {
	return func(s *VolumeService) {
		s.logger = l
	}
}

func WithExchange(e *events.Exchange) NewServiceOption {
	return func(s *VolumeService) {
		s.exchange = e
	}
}

type VolumeService struct {
	volumes.UnimplementedVolumeServiceServer
	local    volumes.VolumeServiceClient
	logger   logger.Logger
	exchange *events.Exchange
}

func (l *VolumeService) Register(server *grpc.Server) error {
	volumes.RegisterVolumeServiceServer(server, l)
	return nil
}

func (l *VolumeService) Get(ctx context.Context, req *volumes.GetRequest) (*volumes.GetResponse, error) {
	return l.local.Get(ctx, req)
}

func (l *VolumeService) List(ctx context.Context, req *volumes.ListRequest) (*volumes.ListResponse, error) {
	return l.local.List(ctx, req)
}

func (l *VolumeService) Create(ctx context.Context, req *volumes.CreateRequest) (*volumes.CreateResponse, error) {
	return l.local.Create(ctx, req)
}

func (l *VolumeService) Delete(ctx context.Context, req *volumes.DeleteRequest) (*volumes.DeleteResponse, error) {
	return l.local.Delete(ctx, req)
}

func (l *VolumeService) Update(ctx context.Context, req *volumes.UpdateRequest) (*volumes.UpdateResponse, error) {
	return l.local.Update(ctx, req)
}

func (l *VolumeService) Patch(ctx context.Context, req *volumes.PatchRequest) (*volumes.PatchResponse, error) {
	return l.local.Patch(ctx, req)
}

func (l *VolumeService) UpdateStatus(ctx context.Context, req *volumes.UpdateStatusRequest) (*volumes.UpdateStatusResponse, error) {
	return l.local.UpdateStatus(ctx, req)
}

func NewService(repo repository.VolumeRepository, opts ...NewServiceOption) *VolumeService {
	s := &VolumeService{
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
