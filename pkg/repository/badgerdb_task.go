package repository

import (
	"context"
	"fmt"

	"github.com/amimof/voiyd/pkg/labels"
	"github.com/amimof/voiyd/pkg/util"
	"github.com/dgraph-io/badger/v4"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/protobuf/proto"

	tasksv1 "github.com/amimof/voiyd/api/services/tasks/v1"
)

var (
	taskPrefix = []byte("task")
	tracer     = otel.GetTracerProvider().Tracer("voiyd-server")
)

type TaskID string

func (c TaskID) String() string {
	return fmt.Sprintf("%s:%s", string(taskPrefix), string(c))
}

type taskBadgerRepo struct {
	db *badger.DB
}

func NewTaskBadgerRepository(db *badger.DB) *taskBadgerRepo {
	return &taskBadgerRepo{
		db: db,
	}
}

func (r *taskBadgerRepo) Get(ctx context.Context, id string) (*tasksv1.Task, error) {
	_, span := tracer.Start(ctx, "repo.task.Get")
	span.SetAttributes(
		attribute.String("service", "Database"),
		attribute.String("provider", "badger"),
		attribute.String("task.id", id),
	)
	defer span.End()

	res := &tasksv1.Task{}
	err := r.db.View(func(txn *badger.Txn) error {
		key := TaskID(id).String()
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

func (r *taskBadgerRepo) List(ctx context.Context, l ...labels.Label) ([]*tasksv1.Task, error) {
	_, span := tracer.Start(ctx, "repo.task.List")
	defer span.End()

	filter := util.MergeLabels(l...)
	var result []*tasksv1.Task
	err := r.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = taskPrefix
		it := txn.NewIterator(opts)
		defer it.Close()

		it.Seek(taskPrefix)
		for it.ValidForPrefix(taskPrefix) {
			item := it.Item()
			task := &tasksv1.Task{}
			err := item.Value(func(val []byte) error {
				err := proto.Unmarshal(val, task)
				if err != nil {
					return err
				}
				return nil
			})
			if err != nil {
				return err
			}
			filter := labels.NewCompositeSelectorFromMap(filter)
			if filter.Matches(task.GetMeta().GetLabels()) {
				result = append(result, task)
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

func (r *taskBadgerRepo) Create(ctx context.Context, task *tasksv1.Task) error {
	_, span := tracer.Start(ctx, "repo.task.Create")
	defer span.End()

	return r.db.Update(func(txn *badger.Txn) error {
		key := TaskID(task.GetMeta().GetName()).String()
		b, err := proto.Marshal(task)
		if err != nil {
			return err
		}
		return txn.Set([]byte(key), b)
	})
}

func (r *taskBadgerRepo) Delete(ctx context.Context, id string) error {
	_, span := tracer.Start(ctx, "repo.container.Delete")
	defer span.End()

	return r.db.Update(func(txn *badger.Txn) error {
		key := TaskID(id).String()
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

func (r *taskBadgerRepo) Update(ctx context.Context, task *tasksv1.Task) error {
	_, span := tracer.Start(ctx, "repo.task.Update")
	defer span.End()

	return r.Create(ctx, task)
}
