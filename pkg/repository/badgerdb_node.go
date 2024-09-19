package repository

import (
	"context"
	"fmt"

	"github.com/amimof/blipblop/api/services/nodes/v1"
	"github.com/dgraph-io/badger/v4"
	"google.golang.org/protobuf/proto"
)

var nodePrefix = []byte("node")

type NodeID string

func (c NodeID) String() string {
	return fmt.Sprintf("%s:%s", string(nodePrefix), string(c))
}

type nodeBadgerRepo struct {
	db *badger.DB
}

func NewNodeBadgerRepository(db *badger.DB) *nodeBadgerRepo {
	return &nodeBadgerRepo{
		db: db,
	}
}

func (r *nodeBadgerRepo) Get(ctx context.Context, id string) (*nodes.Node, error) {
	res := &nodes.Node{}
	err := r.db.View(func(txn *badger.Txn) error {
		key := NodeID(id).String()
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

func (r *nodeBadgerRepo) List(ctx context.Context) ([]*nodes.Node, error) {
	var result []*nodes.Node
	err := r.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		prefixKey := []byte(nodePrefix)
		for it.Seek(prefixKey); it.ValidForPrefix(prefixKey); it.Next() {
			item := it.Item()
			return item.Value(func(val []byte) error {
				ctr := &nodes.Node{}
				err := proto.Unmarshal(val, ctr)
				if err != nil {
					return err
				}
				result = append(result, ctr)
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

func (r *nodeBadgerRepo) Create(ctx context.Context, node *nodes.Node) error {
	return r.db.Update(func(txn *badger.Txn) error {
		key := NodeID(node.GetName()).String()
		b, err := proto.Marshal(node)
		if err != nil {
			return err
		}
		return txn.Set([]byte(key), b)
	})
}

func (r *nodeBadgerRepo) Delete(ctx context.Context, id string) error {
	return r.db.Update(func(txn *badger.Txn) error {
		key := NodeID(id).String()
		return txn.Delete([]byte(key))
	})
}

func (r *nodeBadgerRepo) Update(ctx context.Context, node *nodes.Node) error {
	return r.Create(ctx, node)
}