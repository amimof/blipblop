package lease

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/amimof/voiyd/pkg/auth"
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
	signingKey  *ecdsa.PrivateKey
	tokenStore  map[string]string
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

	leases, err := l.repo.List(ctx, 0)
	if err != nil {
		return nil, l.handleError(err, "error listing leases")
	}

	// Check if lease already exists
	for _, existing := range leases {
		if existing.GetConfig().GetTaskId() == req.GetTaskId() {
			// Allows leases to be stolen if expired
			if time.Now().Before(existing.GetConfig().GetExpiresAt().AsTime().Add(l.gracePeriod)) {
				return nil, status.Error(codes.AlreadyExists, fmt.Sprintf("lease for task %s already exist", req.GetTaskId()))
			}

			return l.updateLease(ctx, existing.GetMeta().GetUid(), req)
		}
	}

	return l.createLease(ctx, req)
}

func (l *local) createLease(ctx context.Context, req *leasesv1.AcquireRequest) (*leasesv1.AcquireResponse, error) {
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

	// Sign JWT access token
	claims := auth.NewLeaseClaim(req.GetTaskId(), req.GetNodeId(), lease.GetMeta().GetUid(), lease.GetConfig().GetAcquiredAt().AsTime())

	// Sign JWT refresh token
	refreshToken, refreshTokenHash, err := auth.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}

	// Persist token to store
	// TODO: Replace with a real store
	l.tokenStore[lease.GetConfig().GetTaskId()] = refreshTokenHash

	// Sign JWT token
	token, err := auth.Generate(claims, l.signingKey)
	if err != nil {
		return nil, err
	}

	newLease, err := l.repo.Create(ctx, lease)
	if err != nil {
		fmt.Println("error creating lease", err)
		return nil, l.handleError(err, "error creating lease", "name", req.GetTaskId())
	}

	err = l.exchange.Publish(ctx, events.NewEvent(events.LeaseAcquiered, newLease))
	if err != nil {
		return nil, l.handleError(err, "publish error",
			"leaseID", newLease.GetMeta().GetName(),
			"task", newLease.GetConfig().GetTaskId(),
			"node", newLease.GetConfig().GetNodeId())
	}
	return &leasesv1.AcquireResponse{Lease: newLease, Token: token, RefreshToken: refreshToken}, nil
}

func (l *local) updateLease(ctx context.Context, leaseID string, req *leasesv1.AcquireRequest) (*leasesv1.AcquireResponse, error) {
	uid, err := keys.ParseStr(leaseID)
	if err != nil {
		return nil, err
	}

	lease, err := l.repo.Get(ctx, uid)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	expires := now.Add(time.Duration(l.leaseTTL) * time.Second)

	lease.GetConfig().AcquiredAt = timestamppb.New(now)
	lease.GetConfig().RenewTime = timestamppb.New(now)
	lease.GetConfig().ExpiresAt = timestamppb.New(expires)
	lease.GetMeta().ResourceVersion++
	lease.GetMeta().Generation++

	// Sign JWT access token
	claims := auth.NewLeaseClaim(req.GetTaskId(), req.GetNodeId(), lease.GetMeta().GetUid(), lease.GetConfig().GetAcquiredAt().AsTime())

	// Sign JWT refresh token
	refreshToken, refreshTokenHash, err := auth.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}

	// Persist token to store
	// TODO: Replace with a real store
	l.tokenStore[lease.GetConfig().GetTaskId()] = refreshTokenHash

	// Sign JWT token
	token, err := auth.Generate(claims, l.signingKey)
	if err != nil {
		return nil, err
	}

	err = l.repo.Update(ctx, uid, lease)
	if err != nil {
		return nil, err
	}

	updatedLease, err := l.repo.Get(ctx, uid)
	if err != nil {
		return nil, err
	}

	err = l.exchange.Publish(ctx, events.NewEvent(events.LeaseAcquiered, updatedLease))
	if err != nil {
		return nil, l.handleError(err, "publish error",
			"leaseID", updatedLease.GetMeta().GetName(),
			"task", updatedLease.GetConfig().GetTaskId(),
			"node", updatedLease.GetConfig().GetNodeId())
	}

	return &leasesv1.AcquireResponse{Lease: updatedLease, Token: token, RefreshToken: refreshToken}, nil
}

func (l *local) Release(ctx context.Context, req *leasesv1.ReleaseRequest, _ ...grpc.CallOption) (*leasesv1.ReleaseResponse, error) {
	// Validate UIDs in req
	if _, err := uuid.Parse(req.GetTaskId()); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	_, err := auth.Verify(req.GetToken(), l.signingKey.PublicKey)
	if err != nil {
		return nil, status.Error(codes.PermissionDenied, "invalid token")
	}

	leases, err := l.repo.List(ctx, 0)
	if err != nil {
		return nil, l.handleError(err, "error listing leases")
	}

	for _, existing := range leases {
		if existing.GetConfig().GetTaskId() == req.GetTaskId() {

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
	existingHash, ok := l.tokenStore[req.GetTaskId()]
	if !ok {
		return nil, status.Error(codes.NotFound, "lease not found")
	}

	if existingHash != auth.HashRefreshToken(req.GetRefreshToken()) {
		return nil, status.Error(codes.InvalidArgument, "invalid refresh token")
	}

	lease, err := l.renew(ctx, req.GetTaskId())
	if err != nil {
		return &leasesv1.RenewResponse{Renewed: false}, err
	}

	refreshToken, refreshTokenHash, err := auth.GenerateRefreshToken()
	if err != nil {
		return nil, status.Errorf(codes.Unknown, "error generating refresh token: %v", err)
	}

	l.tokenStore[req.GetTaskId()] = refreshTokenHash

	err = l.exchange.Publish(ctx, events.NewEvent(events.LeaseRenewed, lease))
	if err != nil {
		return nil, l.handleError(err, "error publishing lease released event", "error", err, "leaseID", lease.GetMeta().GetName(), "task", lease.GetConfig().GetTaskId(), "node", lease.GetConfig().GetNodeId())
	}

	return &leasesv1.RenewResponse{Renewed: true, Lease: lease, RefreshToken: refreshToken}, nil
}

func (l *local) renew(ctx context.Context, taskID string) (*leasesv1.Lease, error) {
	// Validate UIDs in req
	if _, err := uuid.Parse(taskID); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	leases, err := l.repo.List(ctx, 0)
	if err != nil {
		return nil, l.handleError(err, "error listing leases")
	}

	for _, existing := range leases {
		if existing.GetConfig().GetTaskId() == taskID {

			// Renew if lease has a holder which has already expired
			if time.Now().After(existing.GetConfig().GetExpiresAt().AsTime().Add(l.gracePeriod)) {
				return nil, status.Error(codes.FailedPrecondition, "lease expired")
			}

			// Update expiry
			existing.GetConfig().RenewTime = timestamppb.Now()
			existing.GetConfig().ExpiresAt = timestamppb.New(time.Now().Add(time.Duration(existing.GetConfig().GetTtlSeconds()) * time.Second))
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
