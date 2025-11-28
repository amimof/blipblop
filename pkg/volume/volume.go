// Package volume provides an API for working with volumes on nodes
package volume

import (
	"context"

	volumesv1 "github.com/amimof/blipblop/api/services/volumes/v1"
)

type ID string

type MountOpts func(*Driver)

type Driver interface {
	// Create creates a new volume
	Create(ctx context.Context, name string) (Volume, error)

	// Get gets a volume by name
	Get(ctx context.Context, name string) (Volume, error)

	// Delete removes a volume. The volume must be unmounted before removal
	Delete(ctx context.Context, name string) error

	// List lists all valumes managed by the driver
	List(context.Context) ([]Volume, error)

	// Mount mounts the volume as described by the volume type
	Mount(context.Context, Volume, ...MountOpts) error

	// Unmount unmounts the volume as described by the volume type
	Unmount(context.Context, Volume, ...MountOpts) error

	// Snapshot allows for cheap cloning/snapshotting
	Snapshot(context.Context, Volume, ...MountOpts) error
}

type Volume interface {
	ID() ID
	Size() int64
	Location() string
}

type DriverType int32

func (d DriverType) String() string {
	switch d {
	case DriverTypeUnspecified:
		return "unspecified"
	case DriverTypeHostLocal:
		return "host-local"
	}
	return ""
}

const (
	DriverTypeUnspecified DriverType = 0
	DriverTypeHostLocal   DriverType = 1
)

func GetDriverType(vol *volumesv1.Volume) DriverType {
	if vol.GetConfig().GetHostLocal() != nil {
		return DriverTypeHostLocal
	}
	return DriverTypeUnspecified
}
