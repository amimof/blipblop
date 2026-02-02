// Package repository provides interfaces for implementing storage solutions for types
package repository

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/amimof/voiyd/pkg/keys"
	"github.com/dgraph-io/badger/v4"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	containersetsv1 "github.com/amimof/voiyd/api/services/containersets/v1"
	eventsv1 "github.com/amimof/voiyd/api/services/events/v1"
	leasesv1 "github.com/amimof/voiyd/api/services/leases/v1"
	nodesv1 "github.com/amimof/voiyd/api/services/nodes/v1"
	tasksv1 "github.com/amimof/voiyd/api/services/tasks/v1"
	volumesv1 "github.com/amimof/voiyd/api/services/volumes/v1"
	typesv1 "github.com/amimof/voiyd/api/types/v1"
)

var (
	ErrNotFound  = errors.New("item not found")
	ErrIdxExists = errors.New("index already exists")
)

type Txn interface {
	Get(key []byte) ([]byte, error)     // returns a COPY of value
	List([]byte, int) ([][]byte, error) // returns a COPY of value
	Set(key, val []byte) error
	Delete(key []byte) error
	Keys([]byte) ([][]byte, error)
}

type DB interface {
	View(ctx context.Context, fn func(txn Txn) error) error
	Update(ctx context.Context, fn func(txn Txn) error) error
}

var TaskCodec = ProtoCodec[*tasksv1.Task]{
	New: func() *tasksv1.Task { return &tasksv1.Task{} },
}

func NewTaskRepo[T *tasksv1.Task](db DB) *Repo[*tasksv1.Task] {
	return NewRepo(db, TaskCodec, []byte("task/"), []byte("i/task/"))
}

var NodeCodec = ProtoCodec[*nodesv1.Node]{
	New: func() *nodesv1.Node { return &nodesv1.Node{} },
}

func NewNodeRepo[T *nodesv1.Node](db DB) *Repo[*nodesv1.Node] {
	return NewRepo(db, NodeCodec, []byte("node/"), []byte("i/node/"))
}

var VolumeCodec = ProtoCodec[*volumesv1.Volume]{
	New: func() *volumesv1.Volume { return &volumesv1.Volume{} },
}

func NewVolumeRepo[T *volumesv1.Volume](db DB) *Repo[*volumesv1.Volume] {
	return NewRepo(db, VolumeCodec, []byte("volume/"), []byte("i/volume/"))
}

var EventCodec = ProtoCodec[*eventsv1.Event]{
	New: func() *eventsv1.Event { return &eventsv1.Event{} },
}

func NewEventRepo[T *eventsv1.Event](db DB) *Repo[*eventsv1.Event] {
	return NewRepo(db, EventCodec, []byte("event/"), []byte("i/event/"))
}

var LeaseCodec = ProtoCodec[*leasesv1.Lease]{
	New: func() *leasesv1.Lease { return &leasesv1.Lease{} },
}

func NewLeaseRepo[T *leasesv1.Lease](db DB) *Repo[*leasesv1.Lease] {
	return NewRepo(db, LeaseCodec, []byte("lease/"), []byte("i/lease/"))
}

var ContainerSetCodec = ProtoCodec[*containersetsv1.ContainerSet]{
	New: func() *containersetsv1.ContainerSet { return &containersetsv1.ContainerSet{} },
}

func NewContainerSetRepo[T *containersetsv1.ContainerSet](db DB) *Repo[*containersetsv1.ContainerSet] {
	return NewRepo(db, ContainerSetCodec, []byte("containerset/"), []byte("i/containerset/"))
}

type Codec[T proto.Message] interface {
	Decode([]byte) (T, error)
}

type ProtoCodec[T proto.Message] struct {
	New func() T
}

func (c ProtoCodec[T]) Decode(b []byte) (T, error) {
	msg := c.New()
	if err := proto.Unmarshal(b, msg); err != nil {
		var zero T
		return zero, err
	}
	return msg, nil
}

type Repo[T proto.Message] struct {
	// db      *badger.DB
	db      DB
	prefix  []byte
	iprefix []byte
	Codec   Codec[T]
}

// func NewRepo[T proto.Message](db *badger.DB, codec Codec[T], prefix, iprefix []byte) *Repo[T] {
func NewRepo[T proto.Message](db DB, codec Codec[T], prefix, iprefix []byte) *Repo[T] {
	return &Repo[T]{
		db:      db,
		prefix:  prefix,
		iprefix: iprefix,
		Codec:   codec,
	}
}

func (r Repo[T]) List(ctx context.Context, limit int) ([]T, error) {
	var out []T

	err := r.db.View(ctx, func(txn Txn) error {
		kvs, err := txn.List(r.prefix, limit)
		if err != nil {
			return err
		}

		out = make([]T, 0, len(kvs))
		for _, kvp := range kvs {
			obj, derr := r.Codec.Decode(kvp)
			if derr != nil {
				return derr
			}
			out = append(out, obj)
		}
		return nil
	})

	// If out is empty, nextCursor will be nil.
	return out, err
}

func (r Repo[T]) Get(ctx context.Context, id keys.ID) (T, error) {
	var res T

	err := r.db.View(ctx, func(txn Txn) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		switch id.Tag() {
		case keys.TagUID:
			t, err := r.getByUID(ctx, id)
			if err != nil {
				return err
			}
			res = t

		case keys.TagName:
			t, err := r.getByName(ctx, id)
			if err != nil {
				return err
			}
			res = t
		default:
			return fmt.Errorf("unsupported id tag: %v", id.Tag())
		}
		return nil
	})
	return res, err
}

func (r *Repo[T]) getByUID(ctx context.Context, id keys.ID) (T, error) {
	key := id.EncodePrefixed(r.prefix)

	var res T
	err := r.db.View(ctx, func(txn Txn) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		item, err := txn.Get(key)
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return ErrNotFound
			}
			return err
		}
		decoded, err := r.Codec.Decode(item)
		if err != nil {
			return err
		}
		res = decoded
		return nil
	})
	if err != nil {
		return res, err
	}
	return res, nil
}

func (r Repo[T]) getByName(ctx context.Context, id keys.ID) (T, error) {
	name := id.EncodePrefixed(r.iprefix)
	var res T

	err := r.db.View(ctx, func(txn Txn) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		idxItem, err := txn.Get(name)
		if err != nil {
			return err
		}

		var uid keys.ID
		uid, err = keys.Decode(idxItem)
		if err != nil {
			return err
		}

		item, err := txn.Get(uid.EncodePrefixed(r.prefix))
		if err != nil {
			return err
		}
		decoded, err := r.Codec.Decode(item)
		if err != nil {
			return err
		}
		res = decoded
		return nil
	})
	return res, err
}

type Resource interface {
	proto.Message
	GetMeta() *typesv1.Meta
}

func (r *Repo[T]) Create(ctx context.Context, resource Resource) (T, error) {
	var res T

	u := uuid.New()
	resource.GetMeta().Uid = u.String()
	resource.GetMeta().Created = timestamppb.Now()
	resource.GetMeta().Updated = timestamppb.Now()

	uid, err := keys.UUID(u)
	if err != nil {
		return res, err
	}
	name, err := keys.Name(resource.GetMeta().GetName())
	if err != nil {
		return res, err
	}

	b, err := proto.Marshal(resource)
	if err != nil {
		return res, err
	}

	err = r.db.Update(ctx, func(txn Txn) error {
		_, err := txn.Get(name.EncodePrefixed(r.iprefix))
		if err == nil {
			return ErrIdxExists
		}

		if err := txn.Set(uid.EncodePrefixed(r.prefix), b); err != nil {
			return err
		}

		if err := txn.Set(name.EncodePrefixed(r.iprefix), uid.Encode()); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return res, err
	}

	_, err = r.getByUID(ctx, uid)
	if err != nil {
		return res, err
	}

	return res, nil
}

func (r Repo[T]) Delete(ctx context.Context, id keys.ID) error {
	err := r.db.Update(ctx, func(txn Txn) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		switch id.Tag() {
		case keys.TagUID:
			ids, err := txn.Keys(r.iprefix)
			if err != nil {
				return err
			}
			for _, id := range ids {
				if bytes.HasSuffix(id, r.iprefix) {
					if err := txn.Delete(id); err != nil {
						return err
					}
				}
			}
			key := id.EncodePrefixed(r.prefix)
			if err := txn.Delete(key); err != nil {
				return err
			}
		case keys.TagName:
			idxKey := id.EncodePrefixed(r.iprefix)

			idxItem, err := txn.Get(idxKey)
			if err != nil {
				return err
			}

			uid, err := keys.Decode(idxItem)
			if err != nil {
				return err
			}

			if err := txn.Delete(uid.EncodePrefixed(r.prefix)); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported id tag: %v", id.Tag())
		}

		return nil
	})
	return err
}

func (r Repo[T]) Update(ctx context.Context, id keys.ID, resource Resource) error {
	err := r.db.View(ctx, func(txn Txn) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// TODO: Consider returning not-found err and let users decide if they want to override
		// _, err := txn.Get(id.EncodePrefixed(r.prefix))
		// if err == nil {
		// 	return ErrIdxExists
		// }

		switch id.Tag() {
		case keys.TagUID:
			err := r.updateByUID(ctx, id, resource)
			if err != nil {
				return err
			}

		case keys.TagName:
			err := r.updateByName(ctx, id, resource)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported id tag: %v", id.Tag())
		}
		return nil
	})
	return err
}

func (r Repo[T]) updateByName(ctx context.Context, id keys.ID, resource Resource) error {
	resource.GetMeta().Updated = timestamppb.Now()

	b, err := proto.Marshal(resource)
	if err != nil {
		return err
	}

	return r.db.Update(ctx, func(txn Txn) error {
		idxItem, err := txn.Get(id.EncodePrefixed(r.iprefix))
		if err != nil {
			return err
		}

		uid, err := keys.Decode(idxItem)
		if err != nil {
			return err
		}

		if err := txn.Set(uid.EncodePrefixed(r.prefix), b); err != nil {
			return err
		}

		return nil
	})
}

func (r Repo[T]) updateByUID(ctx context.Context, id keys.ID, resource Resource) error {
	resource.GetMeta().Updated = timestamppb.Now()

	b, err := proto.Marshal(resource)
	if err != nil {
		return err
	}

	return r.db.Update(ctx, func(txn Txn) error {
		_, err := txn.Get(id.EncodePrefixed(r.prefix))
		if err != nil {
			return err
		}
		if err := txn.Set(id.EncodePrefixed(r.prefix), b); err != nil {
			return err
		}

		return nil
	})
}
