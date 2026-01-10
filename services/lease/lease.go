package lease

import (
	"context"

	"google.golang.org/grpc"

	"github.com/amimof/voiyd/pkg/events"
	"github.com/amimof/voiyd/pkg/logger"
	"github.com/amimof/voiyd/pkg/repository"

	leasesv1 "github.com/amimof/voiyd/api/services/leases/v1"
)

const Version string = "lease/v1"

type NewServiceOption func(s *LeaseService)

func WithLogger(l logger.Logger) NewServiceOption {
	return func(s *LeaseService) {
		s.logger = l
	}
}

func WithExchange(e *events.Exchange) NewServiceOption {
	return func(s *LeaseService) {
		s.exchange = e
	}
}

type LeaseService struct {
	leasesv1.UnimplementedLeaseServiceServer
	local    leasesv1.LeaseServiceClient
	logger   logger.Logger
	exchange *events.Exchange
}

func (c *LeaseService) Register(server *grpc.Server) error {
	server.RegisterService(&leasesv1.LeaseService_ServiceDesc, c)
	return nil
}

func (c *LeaseService) Get(ctx context.Context, req *leasesv1.GetRequest) (*leasesv1.GetResponse, error) {
	return c.local.Get(ctx, req)
}

func (c *LeaseService) List(ctx context.Context, req *leasesv1.ListRequest) (*leasesv1.ListResponse, error) {
	return c.local.List(ctx, req)
}

func (c *LeaseService) Acquire(ctx context.Context, req *leasesv1.AcquireRequest) (*leasesv1.AcquireResponse, error) {
	return c.local.Acquire(ctx, req)
}

func (c *LeaseService) Release(ctx context.Context, req *leasesv1.ReleaseRequest) (*leasesv1.ReleaseResponse, error) {
	return c.local.Release(ctx, req)
}

func (c *LeaseService) Renew(ctx context.Context, req *leasesv1.RenewRequest) (*leasesv1.RenewResponse, error) {
	return c.local.Renew(ctx, req)
}

func NewService(repo repository.LeaseRepository, opts ...NewServiceOption) *LeaseService {
	s := &LeaseService{
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
