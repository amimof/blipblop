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
	"github.com/amimof/voiyd/pkg/keys"
	"github.com/amimof/voiyd/pkg/logger"
	"github.com/amimof/voiyd/pkg/repository"
	"github.com/google/uuid"

	leasesv1 "github.com/amimof/voiyd/api/services/leases/v1"
	"github.com/amimof/voiyd/api/types/v1"
)

type local struct {
	repo        *repository.Repo[*leasesv1.Lease]
	mu          sync.Mutex
	exchange    *events.Exchange
	logger      logger.Logger
	leaseTTL    uint32
	gracePeriod time.Duration
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

	leases, err := l.repo.List(ctx, 0)
	if err != nil {
		return nil, l.handleError(err, "error listing leases")
	}

	// Check if lease already exists
	for _, existing := range leases {
		if existing.GetConfig().GetTaskId() == req.GetUid() {
			return &leasesv1.GetResponse{
				Lease: existing,
			}, nil
		}
	}

	return nil, status.Errorf(codes.NotFound, "lease for task %s not found", req.GetUid())
}

func (l *local) List(ctx context.Context, req *leasesv1.ListRequest, _ ...grpc.CallOption) (*leasesv1.ListResponse, error) {
	ctx, span := tracer.Start(ctx, "lease.List")
	defer span.End()

	ctrs, err := l.repo.List(ctx, int(req.GetLimit()))
	if err != nil {
		return nil, l.handleError(err, "error listing leases")
	}
	return &leasesv1.ListResponse{
		Leases: ctrs,
	}, nil
}

// Acquire creates a new lease for a task. The task Id and node Id are expected to be uid's and not names.
func (l *local) Acquire(ctx context.Context, req *leasesv1.AcquireRequest, _ ...grpc.CallOption) (*leasesv1.AcquireResponse, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Validate UIDs in req
	if _, err := uuid.Parse(req.GetTaskId()); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if _, err := uuid.Parse(req.GetNodeId()); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	leases, err := l.repo.List(ctx, 0)
	if err != nil {
		return nil, l.handleError(err, "error listing leases")
	}

	// Check if lease already exists
	for _, existing := range leases {
		if existing.GetConfig().GetTaskId() == req.GetTaskId() {

			// Node is same as before
			if existing.GetConfig().GetNodeId() == req.GetNodeId() {
				lease, err := l.renew(ctx, existing.GetConfig().GetTaskId(), req.GetNodeId())
				if err != nil {
					return nil, err
				}
				return &leasesv1.AcquireResponse{
					Lease:    lease,
					Holder:   existing.GetConfig().GetNodeId(),
					Acquired: true,
				}, nil
			}

			// Different node - check if current lease expired + grace period
			if time.Now().After(existing.GetConfig().GetExpiresAt().AsTime().Add(l.gracePeriod)) {
				lease, err := l.renew(ctx, existing.GetConfig().GetTaskId(), req.GetNodeId())
				if err != nil {
					return nil, err
				}
				return &leasesv1.AcquireResponse{
					Lease:    lease,
					Holder:   req.GetNodeId(),
					Acquired: true,
				}, nil
			}

			return nil, status.Error(codes.AlreadyExists, fmt.Sprintf("lease for task %s already exist", req.GetTaskId()))
		}
	}

	ttl := l.leaseTTL
	now := time.Now()
	expires := now.Add(time.Duration(ttl) * time.Second)

	// Create new lease
	lease := &leasesv1.Lease{
		Version: Version,
		Meta: &types.Meta{
			// TODO: Use something else other than task uid for the lease name.
			// Perhaps a combination of task-name and generation.
			Name:            req.GetTaskId(),
			ResourceVersion: 1,
			Generation:      1,
		},
		Config: &leasesv1.LeaseConfig{
			TaskId:     req.TaskId,
			NodeId:     req.NodeId,
			AcquiredAt: timestamppb.New(now),
			RenewTime:  timestamppb.New(now),
			ExpiresAt:  timestamppb.New(expires),
			TtlSeconds: ttl,
		},
	}

	newLease, err := l.repo.Create(ctx, lease)
	if err != nil {
		return nil, l.handleError(err, "error creating lease", "name", req.GetTaskId())
	}

	err = l.exchange.Publish(ctx, events.NewEvent(events.LeaseAcquiered, newLease))
	if err != nil {
		return nil, l.handleError(err, "error publishing lease acquire event", "leaseID", newLease.GetMeta().GetName(), "task", newLease.GetConfig().GetTaskId(), "node", newLease.GetConfig().GetNodeId())
	}

	return &leasesv1.AcquireResponse{Acquired: true, Lease: newLease}, nil
}

func (l *local) Release(ctx context.Context, req *leasesv1.ReleaseRequest, _ ...grpc.CallOption) (*leasesv1.ReleaseResponse, error) {
	// Validate UIDs in req
	if _, err := uuid.Parse(req.GetTaskId()); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if _, err := uuid.Parse(req.GetNodeId()); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	leases, err := l.repo.List(ctx, 0)
	if err != nil {
		return nil, l.handleError(err, "error listing leases")
	}

	for _, existing := range leases {
		if existing.GetConfig().GetTaskId() == req.GetTaskId() {

			// Decline release request if nodeID does match current lease holder
			if existing.GetConfig().GetNodeId() != req.GetNodeId() {
				return nil, status.Error(codes.InvalidArgument, "cannot release lease on behalf of another lease holder")
			}

			uid, err := keys.ParseStr(existing.GetMeta().GetUid())
			if err != nil {
				return nil, l.handleError(err, "error getting lease", "lease", existing.GetMeta().GetUid())
			}

			err = l.repo.Delete(ctx, uid)
			if err != nil {
				return nil, l.handleError(err, "error releasing lease", "lease", existing.GetMeta().GetName())
			}

			err = l.exchange.Publish(ctx, events.NewEvent(events.LeaseReleased, existing))
			if err != nil {
				return nil, l.handleError(err, "error publishing lease released event", "error", err, "leaseID", existing.GetMeta().GetName(), "task", existing.GetConfig().GetTaskId(), "node", existing.GetConfig().GetNodeId())
			}

			return &leasesv1.ReleaseResponse{Released: true}, err
		}
	}

	return nil, status.Errorf(codes.NotFound, "lease for task %s not found", req.GetTaskId())
}

func (l *local) Renew(ctx context.Context, req *leasesv1.RenewRequest, _ ...grpc.CallOption) (*leasesv1.RenewResponse, error) {
	lease, err := l.renew(ctx, req.GetTaskId(), req.GetNodeId())
	if err != nil {
		return &leasesv1.RenewResponse{Renewed: false}, err
	}

	err = l.exchange.Publish(ctx, events.NewEvent(events.LeaseRenewed, lease))
	if err != nil {
		return nil, l.handleError(err, "error publishing lease released event", "error", err, "leaseID", lease.GetMeta().GetName(), "task", lease.GetConfig().GetTaskId(), "node", lease.GetConfig().GetNodeId())
	}

	return &leasesv1.RenewResponse{Renewed: true, Lease: lease}, nil
}

func (l *local) renew(ctx context.Context, taskID, nodeID string) (*leasesv1.Lease, error) {
	// Validate UIDs in req
	if _, err := uuid.Parse(taskID); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if _, err := uuid.Parse(nodeID); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	leases, err := l.repo.List(ctx, 0)
	if err != nil {
		return nil, l.handleError(err, "error listing leases")
	}

	for _, existing := range leases {
		if existing.GetConfig().GetTaskId() == taskID {

			// Renew if lease has a holder which has already expired
			if existing.GetConfig().GetNodeId() != nodeID {
				if time.Now().Before(existing.GetConfig().GetExpiresAt().AsTime().Add(l.gracePeriod)) {
					return nil, fmt.Errorf("lease held by %s", existing.GetConfig().GetNodeId())
				}
			}

			// Update expiry
			existing.GetConfig().RenewTime = timestamppb.Now()
			existing.GetConfig().ExpiresAt = timestamppb.New(time.Now().Add(time.Duration(existing.GetConfig().GetTtlSeconds()) * time.Second))
			existing.GetConfig().NodeId = nodeID
			existing.GetMeta().ResourceVersion++
			existing.GetMeta().Generation++

			uid, err := keys.ParseStr(existing.GetMeta().GetUid())
			if err != nil {
				return nil, err
			}

			err = l.repo.Update(ctx, uid, existing)
			if err != nil {
				return nil, err
			}
			return existing, nil
		}
	}

	return nil, status.Errorf(codes.NotFound, "lease for task %s not found", taskID)
}
