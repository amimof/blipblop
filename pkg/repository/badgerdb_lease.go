package repository

import (
	"context"
	"fmt"

	"github.com/dgraph-io/badger/v4"
	"google.golang.org/protobuf/proto"

	leasesv1 "github.com/amimof/voiyd/api/services/leases/v1"
)

var leasePrefix = []byte("lease")

type LeaseID string

func (c LeaseID) String() string {
	return fmt.Sprintf("%s:%s", string(leasePrefix), string(c))
}

type leaseBadgerRepo struct {
	db *badger.DB
}

func NewLeaseBadgerRepository(db *badger.DB) *leaseBadgerRepo {
	return &leaseBadgerRepo{
		db: db,
	}
}

func (r *leaseBadgerRepo) Get(ctx context.Context, id string) (*leasesv1.Lease, error) {
	_, span := tracer.Start(ctx, "repo.lease.Get")
	defer span.End()

	res := &leasesv1.Lease{}
	err := r.db.View(func(txn *badger.Txn) error {
		key := LeaseID(id).String()
		item, err := txn.Get([]byte(key))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return ErrNotFound
			}
			return err
		}
		return item.Value(func(val []byte) error {
			return proto.Unmarshal(val, res)
		})
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (r *leaseBadgerRepo) List(ctx context.Context) ([]*leasesv1.Lease, error) {
	_, span := tracer.Start(ctx, "repo.lease.List")
	defer span.End()

	var result []*leasesv1.Lease
	err := r.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = leasePrefix
		it := txn.NewIterator(opts)
		defer it.Close()

		it.Seek(leasePrefix)
		for it.ValidForPrefix(leasePrefix) {
			item := it.Item()
			lease := &leasesv1.Lease{}
			err := item.Value(func(val []byte) error {
				err := proto.Unmarshal(val, lease)
				if err != nil {
					return err
				}
				return nil
			})
			if err != nil {
				return err
			}
			result = append(result, lease)
			it.Next()
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (r *leaseBadgerRepo) Create(ctx context.Context, lease *leasesv1.Lease) error {
	_, span := tracer.Start(ctx, "repo.lease.Create")
	defer span.End()

	return r.db.Update(func(txn *badger.Txn) error {
		key := LeaseID(lease.GetConfig().GetTaskId()).String()
		b, err := proto.Marshal(lease)
		if err != nil {
			return err
		}
		return txn.Set([]byte(key), b)
	})
}

func (r *leaseBadgerRepo) Delete(ctx context.Context, id string) error {
	_, span := tracer.Start(ctx, "repo.lease.Delete")
	defer span.End()

	return r.db.Update(func(txn *badger.Txn) error {
		key := LeaseID(id).String()
		err := txn.Delete([]byte(key))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return ErrNotFound
			}
			return err
		}
		return nil
	})
}

func (r *leaseBadgerRepo) Update(ctx context.Context, lease *leasesv1.Lease) error {
	_, span := tracer.Start(ctx, "repo.lease.Update")
	defer span.End()

	return r.Create(ctx, lease)
}
