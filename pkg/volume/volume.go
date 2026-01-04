// Package volume provides an API for working with volumes on nodes
package volume

import (
	"context"

	nodesv1 "github.com/amimof/voiyd/api/services/nodes/v1"
	volumesv1 "github.com/amimof/voiyd/api/services/volumes/v1"
	clientv1 "github.com/amimof/voiyd/pkg/client/volume/v1"
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
	case DriverTypeTemplate:
		return "template"
	}
	return ""
}

const (
	DriverTypeUnspecified DriverType = 0
	DriverTypeHostLocal   DriverType = 1
	DriverTypeTemplate    DriverType = 2
)

func GetDriverType(vol *volumesv1.Volume) DriverType {
	if vol.GetConfig().GetHostLocal() != nil {
		return DriverTypeHostLocal
	}
	if vol.GetConfig().GetTemplate() != nil {
		return DriverTypeTemplate
	}
	return DriverTypeUnspecified
}

// NodeVolumeDrivers returns a list of drivers, each configured according to the provided Node spec.
// If no drivers are configured on the node then the list will be empty
func NodeVolumeDrivers(volumeClient clientv1.ClientV1, n *nodesv1.Node) map[DriverType]Driver {
	result := make(map[DriverType]Driver)
	drivers := n.GetConfig().GetVolumeDrivers()
	if drivers == nil {
		return result
	}

	if drivers.GetHostLocal() != nil {
		result[DriverTypeHostLocal] = NewHostLocalManager(drivers.GetHostLocal().GetRootDir())
	}

	if drivers.GetTemplate() != nil {
		result[DriverTypeTemplate] = NewTemplateDriver(volumeClient, drivers.GetTemplate().GetRootDir())
	}

	return result
}
