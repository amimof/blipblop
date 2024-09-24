package repository

import (
	"context"
	"fmt"

	"github.com/amimof/blipblop/api/services/containers/v1"
	"github.com/dgraph-io/badger/v4"
	"google.golang.org/protobuf/proto"
)

var containerPrefix = []byte("container")

type ContainerID string

func (c ContainerID) String() string {
	return fmt.Sprintf("%s:%s", string(containerPrefix), string(c))
}

type containerBadgerRepo struct {
	db *badger.DB
}

func NewContainerBadgerRepository(db *badger.DB) *containerBadgerRepo {
	return &containerBadgerRepo{
		db: db,
	}
}

func (r *containerBadgerRepo) Get(ctx context.Context, id string) (*containers.Container, error) {
	res := &containers.Container{}
	err := r.db.View(func(txn *badger.Txn) error {
		key := ContainerID(id).String()
		item, err := txn.Get([]byte(key))
		if err != nil {
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

func (r *containerBadgerRepo) List(ctx context.Context) ([]*containers.Container, error) {
	var result []*containers.Container
	err := r.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = containerPrefix
		it := txn.NewIterator(opts)
		defer it.Close()

		it.Seek(containerPrefix)
		for it.ValidForPrefix(containerPrefix) {
			item := it.Item()
			ctr := &containers.Container{}
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

func (r *containerBadgerRepo) Create(ctx context.Context, container *containers.Container) error {
	return r.db.Update(func(txn *badger.Txn) error {
		key := ContainerID(container.GetMeta().GetName()).String()
		b, err := proto.Marshal(container)
		if err != nil {
			return err
		}
		return txn.Set([]byte(key), b)
	})
}

func (r *containerBadgerRepo) Delete(ctx context.Context, id string) error {
	return r.db.Update(func(txn *badger.Txn) error {
		key := ContainerID(id).String()
		return txn.Delete([]byte(key))
	})
}

func (r *containerBadgerRepo) Update(ctx context.Context, container *containers.Container) error {
	return r.Create(ctx, container)
}
