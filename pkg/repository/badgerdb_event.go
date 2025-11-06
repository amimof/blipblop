package repository

import (
	"context"
	"fmt"

	"github.com/amimof/blipblop/api/services/events/v1"
	"github.com/dgraph-io/badger/v4"
	"google.golang.org/protobuf/proto"
)

var eventPrefix = []byte("event")

type EventID string

func (c EventID) String() string {
	return fmt.Sprintf("%s:%s", string(eventPrefix), string(c))
}

type NewEventBadgerRepositoryOption func(r *eventBadgerRepo)

func WithEventBadgerRepositoryMaxItems(max uint64) NewEventBadgerRepositoryOption {
	return func(r *eventBadgerRepo) {
		r.maxItems = max
	}
}

type eventBadgerRepo struct {
	db       *badger.DB
	maxItems uint64
}

func (r *eventBadgerRepo) Get(ctx context.Context, id string) (*events.Event, error) {
	_, span := tracer.Start(ctx, "repo.event.Get")
	defer span.End()

	res := &events.Event{}
	err := r.db.View(func(txn *badger.Txn) error {
		key := EventID(id).String()
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

func (r *eventBadgerRepo) List(ctx context.Context) ([]*events.Event, error) {
	_, span := tracer.Start(ctx, "repo.event.List")
	defer span.End()

	var result []*events.Event
	err := r.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		for it.Seek(eventPrefix); it.ValidForPrefix(eventPrefix); it.Next() {
			item := it.Item()
			return item.Value(func(val []byte) error {
				ctr := &events.Event{}
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

func (r *eventBadgerRepo) Create(ctx context.Context, event *events.Event) error {
	_, span := tracer.Start(ctx, "repo.event.Create")
	defer span.End()

	// Did we hit the limit?
	res, err := r.List(ctx)
	if err != nil {
		return err
	}

	// Remove first in item
	if len(res) >= int(r.maxItems) {
		fmt.Println("REACHED MAX ITEMS")
		delId := res[0]
		_ = r.Delete(ctx, delId.GetMeta().GetName())
	}

	return r.db.Update(func(txn *badger.Txn) error {
		key := EventID(event.GetMeta().GetName()).String()
		b, err := proto.Marshal(event)
		if err != nil {
			return err
		}

		return txn.Set([]byte(key), b)
	})
}

func (r *eventBadgerRepo) Delete(ctx context.Context, id string) error {
	_, span := tracer.Start(ctx, "repo.event.Delete")
	defer span.End()

	return r.db.Update(func(txn *badger.Txn) error {
		key := EventID(id).String()
		return txn.Delete([]byte(key))
	})
}

func (r *eventBadgerRepo) Update(ctx context.Context, event *events.Event) error {
	_, span := tracer.Start(ctx, "repo.event.Update")
	defer span.End()

	return r.Create(ctx, event)
}

func NewEventBadgerRepository(db *badger.DB, opts ...NewEventBadgerRepositoryOption) *eventBadgerRepo {
	r := &eventBadgerRepo{
		db:       db,
		maxItems: 0,
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}
