package lease

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/amimof/voiyd/pkg/events"
	"github.com/amimof/voiyd/pkg/logger"
	"github.com/amimof/voiyd/pkg/repository"

	leasesv1 "github.com/amimof/voiyd/api/services/leases/v1"
)

type local struct {
	repo     repository.LeaseRepository
	mu       sync.Mutex
	exchange *events.Exchange
	logger   logger.Logger

	gracePeriod    time.Duration // 30s
	maxReschedules uint32        // 3
}

var (
	_      leasesv1.LeaseServiceClient = &local{}
	tracer                             = otel.GetTracerProvider().Tracer("voiyd-server")
)

func (l *local) handleError(err error, msg string, keysAndValues ...any) error {
	def := []any{"error", err.Error()}
	def = append(def, keysAndValues...)
	l.logger.Error(msg, def...)
	if errors.Is(err, repository.ErrNotFound) {
		return status.Error(codes.NotFound, err.Error())
	}
	return status.Error(codes.Internal, err.Error())
}

func (l *local) Get(ctx context.Context, req *leasesv1.GetRequest, _ ...grpc.CallOption) (*leasesv1.GetResponse, error) {
	ctx, span := tracer.Start(ctx, "lease.Get")
	defer span.End()

	container, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		return nil, l.handleError(err, "couldn't GET container from repo", "name", req.GetId())
	}
	return &leasesv1.GetResponse{
		Lease: container,
	}, nil
}

func (l *local) List(ctx context.Context, req *leasesv1.ListRequest, _ ...grpc.CallOption) (*leasesv1.ListResponse, error) {
	ctx, span := tracer.Start(ctx, "lease.List")
	defer span.End()

	ctrs, err := l.Repo().List(ctx)
	if err != nil {
		return nil, l.handleError(err, "couldn't LIST containers from repo")
	}
	return &leasesv1.ListResponse{
		Leases: ctrs,
	}, nil
}

func (l *local) Acquire(ctx context.Context, req *leasesv1.AcquireRequest, _ ...grpc.CallOption) (*leasesv1.AcquireResponse, error) {
	// Check if lease already exists
	existing, err := l.repo.Get(ctx, req.TaskId)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {

			// Create new lease
			lease := &leasesv1.Lease{
				TaskId:          req.TaskId,
				NodeId:          req.NodeId,
				AcquiredAt:      timestamppb.Now(),
				RenewTime:       timestamppb.Now(),
				ExpiresAt:       timestamppb.New(time.Now().Add(time.Duration(req.TtlSeconds) * time.Second)),
				TtlSeconds:      req.TtlSeconds,
				RescheduleCount: existing.RescheduleCount, // Preserve counter
				OriginalNode:    existing.OriginalNode,
			}
			err = l.repo.Create(ctx, lease)
			if err != nil {
				return nil, l.handleError(err, "error creating lease", "name", req.TaskId)
			}
			return &leasesv1.AcquireResponse{Acquired: true, Lease: lease}, nil
		}
		return nil, l.handleError(err, "error getting lease", "name", req.TaskId)
	}

	if existing.NodeId == req.NodeId {
		lease, err := l.renew(ctx, existing.GetTaskId(), existing.GetNodeId())
		if err != nil {
			return nil, err
		}
		return &leasesv1.AcquireResponse{
			Lease:    lease,
			Acquired: true,
		}, nil
	}

	// Different node - check if current lease expired + grace period
	if time.Now().Before(existing.GetExpiresAt().AsTime().Add(l.gracePeriod)) {
		// Lease still valid or in grace period
		return &leasesv1.AcquireResponse{
			Acquired: false,
			Holder:   existing.NodeId,
			Lease:    existing,
		}, nil
	}

	return nil, nil
}

func (l *local) Release(ctx context.Context, req *leasesv1.ReleaseRequest, _ ...grpc.CallOption) (*leasesv1.ReleaseResponse, error) {
	lease, err := l.repo.Get(ctx, req.TaskId)
	if err != nil {
		return &leasesv1.ReleaseResponse{Released: false}, err
	}

	if lease.NodeId != req.NodeId {
		// Not the lease holder - already released or taken
		return &leasesv1.ReleaseResponse{Released: false}, nil
	}

	err = l.repo.Delete(ctx, req.TaskId)
	return &leasesv1.ReleaseResponse{Released: true}, err
}

func (l *local) Renew(ctx context.Context, req *leasesv1.RenewRequest, _ ...grpc.CallOption) (*leasesv1.RenewResponse, error) {
	lease, err := l.renew(ctx, req.GetTaskId(), req.GetNodeId())
	if err != nil {
		return &leasesv1.RenewResponse{Renewed: false}, err
	}
	err = l.repo.Update(ctx, lease)
	if err != nil {
		return nil, err
	}
	return &leasesv1.RenewResponse{Renewed: true, Lease: lease}, nil
}

func (l *local) renew(ctx context.Context, taskID, nodeID string) (*leasesv1.Lease, error) {
	existing, err := l.repo.Get(ctx, taskID)
	if err != nil {
		return nil, err
	}

	if existing.NodeId != nodeID {
		return nil, fmt.Errorf("lease held by %s", existing.NodeId)
	}

	// Update expiry
	existing.RenewTime = timestamppb.Now()
	existing.ExpiresAt = timestamppb.New(time.Now().Add(time.Duration(existing.TtlSeconds) * time.Second))

	err = l.repo.Update(ctx, existing)
	if err != nil {
		return nil, err
	}

	return existing, nil
}

func (l *local) Repo() repository.LeaseRepository {
	if l.repo != nil {
		return l.repo
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return repository.NewLeaseInMemRepo()
}
