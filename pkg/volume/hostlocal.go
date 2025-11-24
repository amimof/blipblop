package volume

import (
	"errors"
	"os"
	"path"
	"sync"
)

var (
	_ Manager = &hostLocalManager{}
	_ Volume  = &hostLocalVolume{}
)

type hostLocalManager struct {
	mu       sync.Mutex
	rootPath string
}

type hostLocalVolume struct {
	location string
	name     string
}

// Location implements Volume.
func (h *hostLocalVolume) Location() string {
	return h.location
}

// Name implements Volume.
func (h *hostLocalVolume) Name() string {
	return h.name
}

// Create implements Manager.
func (h *hostLocalManager) Create(volume Volume) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	p := path.Join(h.rootPath, volume.Name())

	return os.MkdirAll(p, 0o750)
}

// Delete implements Manager.
func (h *hostLocalManager) Delete(name string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	p := path.Join(h.rootPath, name)

	return os.RemoveAll(p)
}

// Get implements Manager.
func (h *hostLocalManager) Get(name string) (Volume, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	p := path.Join(h.rootPath, name)

	fstat, err := os.Stat(p)
	if err != nil {
		return nil, err
	}

	if !fstat.IsDir() {
		return nil, errors.New("location is not a directory")
	}

	return &hostLocalVolume{
		location: p,
		name:     fstat.Name(),
	}, nil
}

// List implements Manager.
func (h *hostLocalManager) List() ([]Volume, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	result := []Volume{}
	dirs, err := os.ReadDir(h.rootPath)
	if err != nil {
		return nil, err
	}
	for _, dir := range dirs {
		if dir.IsDir() {
			result = append(result, &hostLocalVolume{name: dir.Name(), location: path.Join(h.rootPath, dir.Name())})
		}
	}

	return result, nil
}

func NewHostLocalManager(rootPath string) Manager {
	return &hostLocalManager{}
}
