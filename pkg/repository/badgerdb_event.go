package repository

import (
	"context"
	"fmt"
	"sort"

	"github.com/dgraph-io/badger/v4"
	"google.golang.org/protobuf/proto"

	eventsv1 "github.com/amimof/voiyd/api/services/events/v1"
)

var eventPrefix = []byte("event")

type EventID string

func (c EventID) String() string {
	return fmt.Sprintf("%s:%s", string(eventPrefix), string(c))
}

type NewEventBadgerRepositoryOption func(r *eventBadgerRepo)

func WithEventBadgerRepositoryLimit(limit uint64) NewEventBadgerRepositoryOption {
	return func(r *eventBadgerRepo) {
		r.limit = limit
	}
}

type eventBadgerRepo struct {
	db    *badger.DB
	limit uint64
}

func (r *eventBadgerRepo) Get(ctx context.Context, id string) (*eventsv1.Event, error) {
	_, span := tracer.Start(ctx, "repo.event.Get")
	defer span.End()

	res := &eventsv1.Event{}
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

func (r *eventBadgerRepo) List(ctx context.Context) ([]*eventsv1.Event, error) {
	_, span := tracer.Start(ctx, "repo.event.List")
	defer span.End()

	var result []*eventsv1.Event
	err := r.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = eventPrefix
		it := txn.NewIterator(opts)
		defer it.Close()

		it.Seek(eventPrefix)
		for it.ValidForPrefix(eventPrefix) {

			item := it.Item()
			event := &eventsv1.Event{}
			err := item.Value(func(val []byte) error {
				err := proto.Unmarshal(val, event)
				if err != nil {
					return err
				}
				return nil
			})
			if err != nil {
				return err
			}
			result = append(result, event)
			it.Next()
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	sortByCreated(result)

	// No limit => return everything
	if r.limit == 0 {
		return result, nil
	}

	// If fewer or equal events than limit, return all
	if uint64(len(result)) <= r.limit {
		return result, nil
	}

	// Return the last n events
	start := len(result) - int(r.limit)
	return result[start:], nil
}

func sortByCreated(in []*eventsv1.Event) {
	sort.Slice(in, func(i, j int) bool {
		return in[i].GetMeta().GetCreated().AsTime().Before(in[j].GetMeta().GetCreated().AsTime())
	})
}

func (r *eventBadgerRepo) Create(ctx context.Context, event *eventsv1.Event) error {
	_, span := tracer.Start(ctx, "repo.event.Create")
	defer span.End()

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

func (r *eventBadgerRepo) Update(ctx context.Context, event *eventsv1.Event) error {
	_, span := tracer.Start(ctx, "repo.event.Update")
	defer span.End()

	return r.Create(ctx, event)
}

func NewEventBadgerRepository(db *badger.DB, opts ...NewEventBadgerRepositoryOption) *eventBadgerRepo {
	r := &eventBadgerRepo{
		db:    db,
		limit: 100,
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}
