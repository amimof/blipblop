package repository

import (
	"context"
	"fmt"

	"github.com/dgraph-io/badger/v4"
	"google.golang.org/protobuf/proto"

	nodesv1 "github.com/amimof/voiyd/api/services/nodes/v1"
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

func (r *nodeBadgerRepo) Get(ctx context.Context, id string) (*nodesv1.Node, error) {
	_, span := tracer.Start(ctx, "repo.node.Get")
	defer span.End()

	res := &nodesv1.Node{}
	err := r.db.View(func(txn *badger.Txn) error {
		key := NodeID(id).String()
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

func (r *nodeBadgerRepo) List(ctx context.Context) ([]*nodesv1.Node, error) {
	_, span := tracer.Start(ctx, "repo.node.List")
	defer span.End()

	var result []*nodesv1.Node
	err := r.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = nodePrefix
		it := txn.NewIterator(opts)
		defer it.Close()

		it.Seek(nodePrefix)
		for it.ValidForPrefix(nodePrefix) {
			item := it.Item()
			node := &nodesv1.Node{}
			err := item.Value(func(val []byte) error {
				err := proto.Unmarshal(val, node)
				if err != nil {
					return err
				}
				return nil
			})
			if err != nil {
				return err
			}
			result = append(result, node)
			it.Next()
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (r *nodeBadgerRepo) Create(ctx context.Context, node *nodesv1.Node) error {
	_, span := tracer.Start(ctx, "repo.node.Create")
	defer span.End()

	return r.db.Update(func(txn *badger.Txn) error {
		key := NodeID(node.GetMeta().GetName()).String()
		b, err := proto.Marshal(node)
		if err != nil {
			return err
		}
		return txn.Set([]byte(key), b)
	})
}

func (r *nodeBadgerRepo) Delete(ctx context.Context, id string) error {
	_, span := tracer.Start(ctx, "repo.node.Delete")
	defer span.End()

	return r.db.Update(func(txn *badger.Txn) error {
		key := NodeID(id).String()
		return txn.Delete([]byte(key))
	})
}

func (r *nodeBadgerRepo) Update(ctx context.Context, node *nodesv1.Node) error {
	_, span := tracer.Start(ctx, "repo.node.Update")
	defer span.End()

	return r.Create(ctx, node)
}
