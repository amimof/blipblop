// Package store provides an interface as well as implementations for storing proto messages to various surfaces.
package store

import (
	"bytes"
	"io"
	"os"
	"path"
	"sync"

	"google.golang.org/protobuf/proto"
)

var (
	_ ProtoStore = &fsProtoStore{}
	_ ProtoStore = &ephemeralProtoStore{}
)

type ProtoStore interface {
	Load(string, proto.Message) error
	Save(string, proto.Message) error
	Delete(string) error
}

type fsProtoStore struct {
	rootDir string
}

func (s *fsProtoStore) Load(id string, m proto.Message) error {
	fName := path.Join(s.rootDir, id)
	b, err := os.ReadFile(fName)
	if err != nil {
		return err
	}
	err = proto.Unmarshal(b, m)
	if err != nil {
		return err
	}
	return nil
}

// Save writes m to FS overwriting any existing occurance of id
func (s *fsProtoStore) Save(id string, m proto.Message) error {
	b, err := proto.Marshal(m)
	if err != nil {
		return nil
	}

	fName := path.Join(s.rootDir, id)

	f, err := os.OpenFile(fName, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}

	reader := bytes.NewReader(b)

	_, err = io.Copy(f, reader)
	if err != nil {
		return err
	}

	return nil
}

func (s *fsProtoStore) Delete(id string) error {
	fName := path.Join(s.rootDir, id)
	return os.Remove(fName)
}

// NewFSProtoStore returns a filesystem based store starting at dir.
func NewFSProtoStore(dir string) (ProtoStore, error) {
	err := os.MkdirAll(dir, 0o755)
	if err != nil {
		return nil, err
	}

	return &fsProtoStore{
		rootDir: dir,
	}, nil
}

type ephemeralProtoStore struct {
	mu    sync.Mutex
	items map[string]proto.Message
}

// Delete implements [ProtoStore].
func (e *ephemeralProtoStore) Delete(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.items, id)
	return nil
}

// Load implements [ProtoStore].
func (e *ephemeralProtoStore) Load(id string, m proto.Message) error {
	e.mu.Lock()
	v, ok := e.items[id]
	e.mu.Unlock()

	if !ok {
		return os.ErrNotExist
	}

	b, err := proto.Marshal(v)
	if err != nil {
		return err
	}

	return proto.Unmarshal(b, m)
}

// Save implements [ProtoStore].
func (e *ephemeralProtoStore) Save(id string, m proto.Message) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.items[id] = proto.Clone(m)
	return nil
}

func NewEphemeralProtoStore() ProtoStore {
	return &ephemeralProtoStore{
		items: map[string]proto.Message{},
	}
}
