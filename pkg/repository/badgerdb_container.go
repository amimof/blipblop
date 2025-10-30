package repository

import (
	"context"
	"fmt"

	"github.com/amimof/blipblop/api/services/containers/v1"
	"github.com/amimof/blipblop/pkg/labels"
	"github.com/amimof/blipblop/pkg/util"
	"github.com/dgraph-io/badger/v4"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/protobuf/proto"
)

var (
	containerPrefix = []byte("container")
	tracer          = otel.GetTracerProvider().Tracer("blipblop-server")
)

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
	_, span := tracer.Start(ctx, "repo.container.Get")
	span.SetAttributes(
		attribute.String("service", "Database"),
		attribute.String("provider", "badger"),
		attribute.String("container.id", id),
	)
	defer span.End()

	res := &containers.Container{}
	err := r.db.View(func(txn *badger.Txn) error {
		key := ContainerID(id).String()
		item, err := txn.Get([]byte(key))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				span.RecordError(err)
				return ErrNotFound
			}
			span.RecordError(err)
			return err
		}
		return item.Value(func(val []byte) error {
			return proto.Unmarshal(val, res)
		})
	})
	if err != nil {
		span.RecordError(err)
		return nil, err
	}
	return res, nil
}

func (r *containerBadgerRepo) List(ctx context.Context, l ...labels.Label) ([]*containers.Container, error) {
	_, span := tracer.Start(ctx, "repo.container.List")
	defer span.End()

	filter := util.MergeLabels(l...)
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
			filter := labels.NewCompositeSelectorFromMap(filter)
			if filter.Matches(ctr.GetMeta().GetLabels()) {
				result = append(result, ctr)
			}
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
	_, span := tracer.Start(ctx, "repo.container.Create")
	defer span.End()

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
	_, span := tracer.Start(ctx, "repo.container.Delete")
	defer span.End()

	return r.db.Update(func(txn *badger.Txn) error {
		key := ContainerID(id).String()
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

func (r *containerBadgerRepo) Update(ctx context.Context, container *containers.Container) error {
	_, span := tracer.Start(ctx, "repo.container.Update")
	defer span.End()

	return r.Create(ctx, container)
}
