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

type eventBadgerRepo struct {
	db *badger.DB
}

func NewEventBadgerRepository(db *badger.DB) *eventBadgerRepo {
	return &eventBadgerRepo{
		db: db,
	}
}

func (r *eventBadgerRepo) Get(ctx context.Context, id string) (*events.Event, error) {
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
	return r.db.Update(func(txn *badger.Txn) error {
		key := EventID(id).String()
		return txn.Delete([]byte(key))
	})
}

func (r *eventBadgerRepo) Update(ctx context.Context, event *events.Event) error {
	return r.Create(ctx, event)
}
