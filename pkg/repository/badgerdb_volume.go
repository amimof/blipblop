package repository

import (
	"context"
	"fmt"

	"github.com/dgraph-io/badger/v4"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/protobuf/proto"

	"github.com/amimof/voiyd/pkg/labels"
	"github.com/amimof/voiyd/pkg/util"

	volumesv1 "github.com/amimof/voiyd/api/services/volumes/v1"
)

var volumePrefix = []byte("volume")

type VolumeID string

func (c VolumeID) String() string {
	return fmt.Sprintf("%s:%s", string(volumePrefix), string(c))
}

type volumeBadgerRepo struct {
	db *badger.DB
}

func NewvolumeBadgerRepository(db *badger.DB) *volumeBadgerRepo {
	return &volumeBadgerRepo{
		db: db,
	}
}

func (r *volumeBadgerRepo) Get(ctx context.Context, id string) (*volumesv1.Volume, error) {
	_, span := tracer.Start(ctx, "repo.volume.Get")
	span.SetAttributes(
		attribute.String("service", "Database"),
		attribute.String("provider", "badger"),
		attribute.String("volume.id", id),
	)
	defer span.End()

	res := &volumesv1.Volume{}
	err := r.db.View(func(txn *badger.Txn) error {
		key := VolumeID(id).String()
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

func (r *volumeBadgerRepo) List(ctx context.Context, l ...labels.Label) ([]*volumesv1.Volume, error) {
	_, span := tracer.Start(ctx, "repo.volume.List")
	defer span.End()

	filter := util.MergeLabels(l...)
	var result []*volumesv1.Volume
	err := r.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = volumePrefix
		it := txn.NewIterator(opts)
		defer it.Close()

		it.Seek(volumePrefix)
		for it.ValidForPrefix(volumePrefix) {
			item := it.Item()
			vol := &volumesv1.Volume{}
			err := item.Value(func(val []byte) error {
				err := proto.Unmarshal(val, vol)
				if err != nil {
					return err
				}
				return nil
			})
			if err != nil {
				return err
			}
			filter := labels.NewCompositeSelectorFromMap(filter)
			if filter.Matches(vol.GetMeta().GetLabels()) {
				result = append(result, vol)
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

func (r *volumeBadgerRepo) Create(ctx context.Context, volume *volumesv1.Volume) error {
	_, span := tracer.Start(ctx, "repo.volume.Create")
	defer span.End()

	return r.db.Update(func(txn *badger.Txn) error {
		key := VolumeID(volume.GetMeta().GetName()).String()
		b, err := proto.Marshal(volume)
		if err != nil {
			return err
		}
		return txn.Set([]byte(key), b)
	})
}

func (r *volumeBadgerRepo) Delete(ctx context.Context, id string) error {
	_, span := tracer.Start(ctx, "repo.volume.Delete")
	defer span.End()

	return r.db.Update(func(txn *badger.Txn) error {
		key := VolumeID(id).String()
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

func (r *volumeBadgerRepo) Update(ctx context.Context, volume *volumesv1.Volume) error {
	_, span := tracer.Start(ctx, "repo.volume.Update")
	defer span.End()

	return r.Create(ctx, volume)
}

func NewVolumeBadgerRepository(db *badger.DB) *volumeBadgerRepo {
	return &volumeBadgerRepo{
		db: db,
	}
}
