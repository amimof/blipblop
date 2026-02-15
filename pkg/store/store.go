package store

import (
	"bytes"
	"io"
	"os"
	"path"
)

var _ Store = &fsStore{}

type Store interface {
	Load(string) ([]byte, error)
	Save(string, []byte) error
	Delete(string) error
}

type fsStore struct {
	rootDir string
}

// Delete implements [Store].
func (f *fsStore) Delete(id string) error {
	fName := path.Join(f.rootDir, id)
	return os.Remove(fName)
}

// Load implements [Store].
func (f *fsStore) Load(id string) ([]byte, error) {
	fName := path.Join(f.rootDir, id)
	b, err := os.ReadFile(fName)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// Save implements [Store].
func (f *fsStore) Save(id string, data []byte) error {
	fName := path.Join(f.rootDir, id)

	file, err := os.OpenFile(fName, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}

	reader := bytes.NewReader(data)

	_, err = io.Copy(file, reader)
	if err != nil {
		return err
	}

	return nil
}

// NewFSStore returns a filesystem based store starting at dir.
func NewFSStore(dir string) (Store, error) {
	err := os.MkdirAll(dir, 0o755)
	if err != nil {
		return nil, err
	}

	return &fsStore{
		rootDir: dir,
	}, nil
}
