package repository

import (
	"context"
	"fmt"

	"github.com/amimof/blipblop/api/services/nodes/v1"
	"github.com/dgraph-io/badger/v4"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/proto"
)

var nodePrefix = []byte("node")

type NodeID string

func (c NodeID) String() string {
	return fmt.Sprintf("%s:%s", string(nodePrefix), string(c))
}

type nodeBadgerRepo struct {
	db     *badger.DB
	tracer trace.Tracer
}

func NewNodeBadgerRepository(db *badger.DB) *nodeBadgerRepo {
	return &nodeBadgerRepo{
		db:     db,
		tracer: otel.Tracer("blipblop/nodes"),
	}
}

func (r *nodeBadgerRepo) Get(ctx context.Context, id string) (*nodes.Node, error) {
	ctx, span := r.tracer.Start(ctx, "badger.Get")
	defer span.End()

	res := &nodes.Node{}
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

func (r *nodeBadgerRepo) List(ctx context.Context) ([]*nodes.Node, error) {
	ctx, span := r.tracer.Start(ctx, "badger.List")
	defer span.End()

	var result []*nodes.Node
	err := r.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = nodePrefix
		it := txn.NewIterator(opts)
		defer it.Close()

		it.Seek(nodePrefix)
		for it.ValidForPrefix(nodePrefix) {
			item := it.Item()
			ctr := &nodes.Node{}
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

func (r *nodeBadgerRepo) Create(ctx context.Context, node *nodes.Node) error {
	ctx, span := r.tracer.Start(ctx, "badger.Create")
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
	ctx, span := r.tracer.Start(ctx, "badger.Delete")
	defer span.End()

	return r.db.Update(func(txn *badger.Txn) error {
		key := NodeID(id).String()
		return txn.Delete([]byte(key))
	})
}

func (r *nodeBadgerRepo) Update(ctx context.Context, node *nodes.Node) error {
	return r.Create(ctx, node)
}
