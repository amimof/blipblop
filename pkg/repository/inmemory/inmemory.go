// Package inmemory provides a inmemory based repository implementation for testing
package inmemory

import (
	"context"
	"maps"
	"sync"

	"github.com/amimof/voiyd/pkg/repository"
)

type DB struct {
	mu sync.RWMutex
	m  map[string][]byte
}

func New() *DB {
	return &DB{
		m: make(map[string][]byte),
	}
}

type viewTxn struct {
	// snapshot is an immutable view of the DB at View() start
	snapshot map[string][]byte
}

// Keys implements [repository.Txn].
func (t viewTxn) Keys([]byte) ([][]byte, error) {
	out := make([][]byte, 0, len(t.snapshot))
	for k := range t.snapshot {
		out = append(out, []byte(k))
	}
	return out, nil
}

func (t viewTxn) Get(key []byte) ([]byte, error) {
	v, ok := t.snapshot[string(key)]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return clone(v), nil
}

func (t viewTxn) Set(_ []byte, _ []byte) error {
	panic("set called in read-only txn")
}

func (t viewTxn) Delete(_ []byte) error {
	panic("delete called in read-only txn")
}

type updateTxn struct {
	base    map[string][]byte
	writes  map[string][]byte
	deletes map[string]struct{}
}

// Keys implements [repository.Txn].
func (t *updateTxn) Keys([]byte) ([][]byte, error) {
	panic("unimplemented")
}

func (t viewTxn) List(prefix []byte, limit int) ([][]byte, error) {
	out := make([][]byte, 0, len(t.snapshot))
	for _, v := range t.snapshot {
		out = append(out, v)
	}
	return out, nil
}

func (t *updateTxn) Get(key []byte) ([]byte, error) {
	k := string(key)

	if _, del := t.deletes[k]; del {
		return nil, repository.ErrNotFound
	}
	if v, ok := t.writes[k]; ok {
		return clone(v), nil
	}
	if v, ok := t.base[k]; ok {
		return clone(v), nil
	}
	return nil, repository.ErrNotFound
}

func (t *updateTxn) Set(key []byte, val []byte) error {
	k := string(key)
	delete(t.deletes, k)
	t.writes[k] = clone(val)
	return nil
}

func (t *updateTxn) Delete(key []byte) error {
	k := string(key)
	delete(t.writes, k)
	t.deletes[k] = struct{}{}
	return nil
}

func (t *updateTxn) List(prefix []byte, limit int) ([][]byte, error) {
	panic("list called in update-only txn")
}

func (db *DB) View(ctx context.Context, fn func(txn repository.Txn) error) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	db.mu.RLock()
	// create a snapshot map that shares underlying value slices (safe because we clone on Get)
	// If you want super strict immutability, deep-clone values here too (slower).
	snap := make(map[string][]byte, len(db.m))
	maps.Copy(snap, db.m)
	db.mu.RUnlock()

	return fn(viewTxn{snapshot: snap})
}

func (db *DB) Update(ctx context.Context, fn func(txn repository.Txn) error) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	db.mu.RLock()
	base := db.m // do not modify directly
	db.mu.RUnlock()

	txn := &updateTxn{
		base:    base,
		writes:  make(map[string][]byte),
		deletes: make(map[string]struct{}),
	}

	if err := fn(txn); err != nil {
		return err // rollback by dropping txn
	}

	// commit
	db.mu.Lock()
	defer db.mu.Unlock()

	// Apply deletes
	for k := range txn.deletes {
		delete(db.m, k)
	}
	// Apply writes
	maps.Copy(db.m, txn.writes)

	return nil
}

func clone(b []byte) []byte {
	if b == nil {
		return nil
	}
	out := make([]byte, len(b))
	copy(out, b)
	return out
}

func (db *DB) Len() int {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return len(db.m)
}
