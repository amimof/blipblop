package app

import (
	"context"
	"time"

	leasesv1 "github.com/amimof/voiyd/api/services/leases/v1"
)

type (
	ResourceID   string
	HolderID     string
	FencingToken uint64
)

type LeaseGuard interface {
	IsHolder(ctx context.Context, resourceID ResourceID, holderID HolderID) (bool, error)
	AssertHolder(ctx context.Context, resourceID ResourceID, holderID HolderID) (token FencingToken, err error)
}

type LeaseStore interface {
	LeaseGuard
	Acquire(ctx context.Context, resource ResourceID, holder HolderID, ttl time.Duration) (*leasesv1.Lease, string, error)
	Renew(ctx context.Context, resource ResourceID, holder HolderID, ttl time.Duration, token string) (*leasesv1.Lease, string, error)
	Release(ctx context.Context, resource ResourceID, holder HolderID) error
	Get(ctx context.Context, resource ResourceID) (*leasesv1.Lease, error)
	List(ctx context.Context) ([]*leasesv1.Lease, error)
}
