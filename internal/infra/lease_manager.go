package infra

import (
	"context"
	"sync"
	"time"

	"github.com/amimof/voiyd/internal/app"
	"github.com/amimof/voiyd/internal/domain"
	"github.com/amimof/voiyd/pkg/keys"
	"github.com/amimof/voiyd/pkg/repository"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	leasesv1 "github.com/amimof/voiyd/api/services/leases/v1"
	"github.com/amimof/voiyd/api/types/v1"
	"github.com/amimof/voiyd/pkg/errs"
)

var _ app.LeaseStore = &LeaseManager{}

type LeaseManager struct {
	mu  sync.Mutex
	db  *repository.Repo[*leasesv1.Lease]
	TTL uint32
}

// List implements [app.LeaseStore].
func (m *LeaseManager) List(ctx context.Context) ([]*leasesv1.Lease, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.db.List(ctx, 0)
}

// Acquire implements [app.LeaseStore].
func (m *LeaseManager) Acquire(ctx context.Context, resource app.ResourceID, holder app.HolderID, ttl time.Duration) (*leasesv1.Lease, error) {
	if err := ctx.Err(); err != nil {
		return &leasesv1.Lease{}, err
	}
	if holder == "" {
		return &leasesv1.Lease{}, domain.ErrInvalidHolder
	}
	if ttl <= 0 {
		return &leasesv1.Lease{}, domain.ErrInvalidTTL
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	uid, err := keys.Index(string(resource))
	if err != nil {
		return nil, err
	}

	now := time.Now()
	expires := now.Add(time.Duration(m.TTL) * time.Second)

	cur, err := m.db.Get(ctx, uid)
	if err != nil {
		// Either not found, expired, or same holder reacquiring.
		// Bump token when taking ownership from "nobody" (expired/not found) or when switching holders.
		if errs.IsNotFound(err) {

			lease := &leasesv1.Lease{
				Version: string(app.VersionLeaseV1),
				Meta: &types.Meta{
					Name:            string(resource),
					ResourceVersion: 1,
					Generation:      1,
				},
				Config: &leasesv1.LeaseConfig{
					TaskId:       string(resource),
					NodeId:       string(holder),
					AcquiredAt:   timestamppb.New(now),
					RenewTime:    timestamppb.New(now),
					ExpiresAt:    timestamppb.New(expires),
					TtlSeconds:   m.TTL,
					FencingToken: 1,
				},
			}

			return m.db.Create(ctx, lease)
		}

		return nil, err
	}

	// Either not found, expired, or same holder reacquiring.
	// Bump token when taking ownership from "nobody" (expired/not found) or when switching holders.
	if !cur.GetConfig().GetExpiresAt().AsTime().After(now) || cur.GetConfig().GetNodeId() == string(holder) {
		cur.GetConfig().FencingToken++
		cur.GetConfig().RenewTime = timestamppb.New(now)
		cur.GetConfig().AcquiredAt = timestamppb.New(now)
		cur.GetConfig().ExpiresAt = timestamppb.New(expires)
		return m.db.Update(ctx, uid, cur)
	}

	return nil, errs.ErrLeaseHeld
}

// Get implements [app.LeaseStore].
func (m *LeaseManager) Get(ctx context.Context, resource app.ResourceID) (*leasesv1.Lease, error) {
	if err := ctx.Err(); err != nil {
		return &leasesv1.Lease{}, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// cur, ok := m.leases[resource]
	// if !ok {
	// 	return &leasesv1.Lease{}, app.ErrLeaseNotFound
	// }
	// return cur, nil

	uid, err := keys.Index(string(resource))
	if err != nil {
		return nil, err
	}

	return m.db.Get(ctx, uid)
}

// Release implements [app.LeaseStore].
func (m *LeaseManager) Release(ctx context.Context, resource app.ResourceID, holder app.HolderID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if holder == "" {
		return domain.ErrInvalidHolder
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	uid, err := keys.ParseStr(string(resource))
	if err != nil {
		return err
	}

	cur, err := m.db.Get(ctx, uid)
	if err != nil {
		return err
	}

	// cur, ok := m.leases[resource]
	// if !ok {
	// 	return app.ErrLeaseNotFound
	// }

	if app.HolderID(cur.GetConfig().GetNodeId()) != holder {
		return domain.ErrNotHolder
	}

	// delete(m.leases, resource)
	return m.db.Delete(ctx, uid)
}

// Renew implements [app.LeaseStore].
func (m *LeaseManager) Renew(ctx context.Context, resource app.ResourceID, holder app.HolderID, ttl time.Duration, token uint64) (*leasesv1.Lease, error) {
	if err := ctx.Err(); err != nil {
		return &leasesv1.Lease{}, err
	}
	if holder == "" {
		return &leasesv1.Lease{}, domain.ErrInvalidHolder
	}
	if ttl <= 0 {
		return &leasesv1.Lease{}, domain.ErrInvalidTTL
	}

	now := time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()

	uid, err := uuid.Parse(string(resource))
	if err != nil {
		return nil, err
	}

	id, err := keys.Index(uid.String())
	if err != nil {
		return nil, err
	}

	cur, err := m.db.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	if cur.GetConfig().GetFencingToken() != token {
		return &leasesv1.Lease{}, domain.ErrNotHolder
	}

	// Update expiry, bump fencing token
	expires := now.Add(time.Duration(m.TTL) * time.Second)
	cur.GetConfig().FencingToken++
	cur.GetConfig().RenewTime = timestamppb.New(now)
	cur.GetConfig().AcquiredAt = timestamppb.New(now)
	cur.GetConfig().ExpiresAt = timestamppb.New(expires)

	return m.db.Update(ctx, id, cur)
}

func (m *LeaseManager) AssertHolder(ctx context.Context, resourceID app.ResourceID, holderID app.HolderID) (token app.FencingToken, err error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if holderID == "" {
		return 0, domain.ErrInvalidHolder
	}

	now := time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()

	uid, err := uuid.Parse(string(resourceID))
	if err != nil {
		return 0, err
	}

	id, err := keys.UUID(uid)
	if err != nil {
		return 0, err
	}

	cur, err := m.db.Get(ctx, id)
	if err != nil {
		return 0, err
	}
	if !cur.GetConfig().GetExpiresAt().AsTime().After(now) {
		return 0, domain.ErrLeaseExpired
	}
	if app.HolderID(cur.GetConfig().GetNodeId()) != holderID {
		return 0, domain.ErrNotHolder
	}

	return app.FencingToken(cur.GetConfig().GetFencingToken()), nil
}

func NewLeaseManager(repo *repository.Repo[*leasesv1.Lease]) *LeaseManager {
	return &LeaseManager{db: repo, TTL: 60}
}
