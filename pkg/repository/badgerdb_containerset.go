package repository

import (
	"context"
	"fmt"

	containersetsv1 "github.com/amimof/blipblop/api/services/containersets/v1"
	"github.com/dgraph-io/badger/v4"
	"google.golang.org/protobuf/proto"
)

var containerSetPrefix = []byte("set")

type ContainerSetID string

func (c ContainerSetID) String() string {
	return fmt.Sprintf("%s:%s", string(containerSetPrefix), string(c))
}

type containerSetBadgerRepo struct {
	db *badger.DB
}

func NewContainerSetBadgerRepository(db *badger.DB) *containerSetBadgerRepo {
	return &containerSetBadgerRepo{
		db: db,
	}
}

func (r *containerSetBadgerRepo) Get(ctx context.Context, id string) (*containersetsv1.ContainerSet, error) {
	_, span := tracer.Start(ctx, "repo.containerset.Get")
	defer span.End()

	res := &containersetsv1.ContainerSet{}
	err := r.db.View(func(txn *badger.Txn) error {
		key := ContainerSetID(id).String()
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

func (r *containerSetBadgerRepo) List(ctx context.Context) ([]*containersetsv1.ContainerSet, error) {
	_, span := tracer.Start(ctx, "repo.containerset.List")
	defer span.End()

	var result []*containersetsv1.ContainerSet
	err := r.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = containerSetPrefix
		it := txn.NewIterator(opts)
		defer it.Close()

		it.Seek(containerSetPrefix)
		for it.ValidForPrefix(containerSetPrefix) {
			item := it.Item()
			ctr := &containersetsv1.ContainerSet{}
			err := item.Value(func(val []byte) error {
				err := proto.Unmarshal(val, ctr)
				if err != nil {
					return err
				}
				return nil
			})
			if err != nil {
				return err
			}
			result = append(result, ctr)
			it.Next()
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (r *containerSetBadgerRepo) Create(ctx context.Context, container *containersetsv1.ContainerSet) error {
	_, span := tracer.Start(ctx, "repo.containerset.Create")
	defer span.End()

	return r.db.Update(func(txn *badger.Txn) error {
		key := ContainerSetID(container.GetMeta().GetName()).String()
		b, err := proto.Marshal(container)
		if err != nil {
			return err
		}
		return txn.Set([]byte(key), b)
	})
}

func (r *containerSetBadgerRepo) Delete(ctx context.Context, id string) error {
	_, span := tracer.Start(ctx, "repo.containerset.Delete")
	defer span.End()

	return r.db.Update(func(txn *badger.Txn) error {
		key := ContainerSetID(id).String()
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

func (r *containerSetBadgerRepo) Update(ctx context.Context, container *containersetsv1.ContainerSet) error {
	_, span := tracer.Start(ctx, "repo.containerset.Update")
	defer span.End()

	return r.Create(ctx, container)
}
