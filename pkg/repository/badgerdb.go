package repository

import (
	"context"
	"fmt"

	"github.com/dgraph-io/badger/v4"
)

var containerPrefix = []byte("container")

type ContainerID string

func (c ContainerID) String() string {
	return fmt.Sprintf("%s:%s", string(containerPrefix), string(c))
}

type BadgerRepository struct {
	db *badger.DB
}

func NewBadgerRepository(db *badger.DB) *BadgerRepository {
	return &BadgerRepository{
		db: db,
	}
}

func (r *BadgerRepository) Get(ctx context.Context, id string) ([]byte, error) {
	var res []byte
	err := r.db.View(func(txn *badger.Txn) error {
		key := ContainerID(id).String()
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			res = val
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (r *BadgerRepository) GetAll(ctx context.Context) ([][]byte, error) {
	var result [][]byte
	err := r.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		for it.Seek(containerPrefix); it.ValidForPrefix(containerPrefix); it.Next() {
			item := it.Item()
			return item.Value(func(val []byte) error {
				result = append(result, val)
				return nil
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (r *BadgerRepository) Create(ctx context.Context, id string, b []byte) error {
	if id == "" {
		return fmt.Errorf("id cannot be empty")
	}
	return r.db.Update(func(txn *badger.Txn) error {
		key := ContainerID(id).String()
		return txn.Set([]byte(key), b)
	})
}

func (r *BadgerRepository) Delete(ctx context.Context, id string) error {
	return r.db.Update(func(txn *badger.Txn) error {
		key := ContainerID(id).String()
		return txn.Delete([]byte(key))
	})
}

func (r *BadgerRepository) Update(ctx context.Context, id string, data []byte) error {
	return r.Create(ctx, id, data)
}
