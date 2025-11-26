package volume

import (
	"context"
	"fmt"
	"os"
	"path"
	"sync"
)

var _ Driver = &hostLocalDriver{}

type FSVolume struct {
	id       ID
	size     int64
	location string
}

func (fs *FSVolume) ID() ID {
	return ID(fs.id)
}

func (fs *FSVolume) Size() int64 {
	return fs.size
}

func (fs *FSVolume) Location() string {
	return fs.location
}

type hostLocalDriver struct {
	mu       sync.Mutex
	rootPath string
}

// Mount implements Driver.
func (h *hostLocalDriver) Mount(context.Context, Volume, ...MountOpts) error {
	panic("unimplemented")
}

// Snapshot implements Driver.
func (h *hostLocalDriver) Snapshot(context.Context, Volume, ...MountOpts) error {
	panic("unimplemented")
}

// Unmount implements Driver.
func (h *hostLocalDriver) Unmount(context.Context, Volume, ...MountOpts) error {
	panic("unimplemented")
}

// Create implements Manager.
func (h *hostLocalDriver) Create(ctx context.Context, name string) (Volume, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	p := path.Join(h.rootPath, name)

	err := os.MkdirAll(p, 0o750)
	if err != nil {
		return nil, err
	}

	stat, err := os.Stat(p)
	if err != nil {
		return nil, err
	}

	return &FSVolume{
		id:       ID(name),
		size:     stat.Size(),
		location: p,
	}, nil
}

// Delete implements Manager.
func (h *hostLocalDriver) Delete(ctx context.Context, name string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	p := path.Join(h.rootPath, name)

	return os.RemoveAll(p)
}

// Get implements Manager.
func (h *hostLocalDriver) Get(ctx context.Context, name string) (Volume, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	p := path.Join(h.rootPath, name)

	fstat, err := os.Stat(p)
	if err != nil {
		return nil, err
	}

	id := ID(fstat.Name())

	if !fstat.IsDir() {
		return nil, fmt.Errorf("volume location %s is not a directory", p)
	}

	return &FSVolume{
		id:       id,
		location: p,
		size:     fstat.Size(),
	}, nil
}

// List implements Manager.
func (h *hostLocalDriver) List(ctx context.Context) ([]Volume, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	result := []Volume{}
	dirs, err := os.ReadDir(h.rootPath)
	if err != nil {
		return nil, err
	}

	for _, dir := range dirs {
		if dir.IsDir() {
			vol, err := h.Get(ctx, dir.Name())
			if err != nil {
				return nil, err
			}
			result = append(result, vol)
		}
	}

	return result, nil
}

func NewHostLocalManager(rootPath string) Driver {
	return &hostLocalDriver{
		rootPath: rootPath,
	}
}
