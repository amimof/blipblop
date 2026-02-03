package volume

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/amimof/voiyd/pkg/events"
	"github.com/amimof/voiyd/pkg/logger"
	"github.com/amimof/voiyd/pkg/repository"

	volumesv1 "github.com/amimof/voiyd/api/services/volumes/v1"
)

const Version string = "volume/v1"

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
	volumesv1.UnimplementedVolumeServiceServer
	local    volumesv1.VolumeServiceClient
	logger   logger.Logger
	exchange *events.Exchange
}

func (l *VolumeService) Register(server *grpc.Server) error {
	volumesv1.RegisterVolumeServiceServer(server, l)
	return nil
}

func (l *VolumeService) Get(ctx context.Context, req *volumesv1.GetRequest) (*volumesv1.GetResponse, error) {
	return l.local.Get(ctx, req)
}

func (l *VolumeService) List(ctx context.Context, req *volumesv1.ListRequest) (*volumesv1.ListResponse, error) {
	return l.local.List(ctx, req)
}

func (l *VolumeService) Create(ctx context.Context, req *volumesv1.CreateRequest) (*volumesv1.CreateResponse, error) {
	return l.local.Create(ctx, req)
}

func (l *VolumeService) Delete(ctx context.Context, req *volumesv1.DeleteRequest) (*emptypb.Empty, error) {
	return l.local.Delete(ctx, req)
}

func (l *VolumeService) Update(ctx context.Context, req *volumesv1.UpdateRequest) (*volumesv1.UpdateResponse, error) {
	return l.local.Update(ctx, req)
}

func (l *VolumeService) Patch(ctx context.Context, req *volumesv1.PatchRequest) (*volumesv1.PatchResponse, error) {
	return l.local.Patch(ctx, req)
}

func (l *VolumeService) UpdateStatus(ctx context.Context, req *volumesv1.UpdateStatusRequest) (*volumesv1.UpdateStatusResponse, error) {
	return l.local.UpdateStatus(ctx, req)
}

func NewService(repo *repository.Repo[*volumesv1.Volume], opts ...NewServiceOption) *VolumeService {
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
