package app

import (
	"context"
	"crypto/ecdsa"
	"sync"
	"time"

	leasesv1 "github.com/amimof/voiyd/api/services/leases/v1"
	"github.com/amimof/voiyd/pkg/events"
	"github.com/amimof/voiyd/pkg/logger"
)

type LeaseService struct {
	mu          sync.Mutex
	Exchange    *events.Exchange
	Logger      logger.Logger
	LeaseTTL    uint32
	GracePeriod time.Duration
	SigningKey  *ecdsa.PrivateKey
	TokenStore  map[string]string
	Manager     LeaseStore
}

func (l *LeaseService) Get(ctx context.Context, taskID ResourceID) (*leasesv1.Lease, error) {
	ctx, span := tracer.Start(ctx, "lease.Get")
	defer span.End()

	return l.Manager.Get(ctx, taskID)
}

func (l *LeaseService) List(ctx context.Context) ([]*leasesv1.Lease, error) {
	ctx, span := tracer.Start(ctx, "lease.List")
	defer span.End()

	return l.Manager.List(ctx)
}

func (l *LeaseService) Acquire(ctx context.Context, taskID ResourceID, nodeID HolderID) (*leasesv1.Lease, string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.Manager.Acquire(ctx, taskID, nodeID, time.Duration(l.LeaseTTL))
}

func (l *LeaseService) Release(ctx context.Context, taskID ResourceID, holderID HolderID) error {
	return l.Manager.Release(ctx, taskID, holderID)
}

func (l *LeaseService) Renew(ctx context.Context, taskID ResourceID, nodeID HolderID, token string) (*leasesv1.Lease, string, error) {
	return l.Manager.Renew(ctx, taskID, nodeID, time.Duration(l.LeaseTTL), token)
}
