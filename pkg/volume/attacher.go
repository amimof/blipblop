package volume

import (
	"context"
	"fmt"
	"os"

	containersv1 "github.com/amimof/voiyd/api/services/containers/v1"
	nodesv1 "github.com/amimof/voiyd/api/services/nodes/v1"
	volumesv1 "github.com/amimof/voiyd/api/services/volumes/v1"
	clientv1 "github.com/amimof/voiyd/pkg/client/volume/v1"
	"github.com/amimof/voiyd/pkg/consts"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var _ Attacher = &DefaultAttacher{}

type Attacher interface {
	// PrepareMounts inspects ctr.Config.Mounts, resolves any mount.Volume
	// to a local host path using the appropriate Driver, and updates the
	// mounts with mount.Source = host path.
	PrepareMounts(ctx context.Context, node *nodesv1.Node, ctr *containersv1.Container) error

	// Allows for reversal of provisioned resources
	Detach(ctx context.Context, node *nodesv1.Node, ctr *containersv1.Container) error
}

type DefaultAttacher struct {
	volumeClient clientv1.ClientV1
}

// NewDefaultAttacher returns a new default attacher
func NewDefaultAttacher(vc clientv1.ClientV1) *DefaultAttacher {
	return &DefaultAttacher{volumeClient: vc}
}

// PrepareMounts implements Attacher interface
func (a *DefaultAttacher) PrepareMounts(ctx context.Context, n *nodesv1.Node, ctr *containersv1.Container) error {
	mounts := ctr.GetConfig().GetMounts()
	if len(mounts) == 0 {
		return nil
	}

	nodeName := n.GetMeta().GetName()
	drivers := NodeVolumeDrivers(a.volumeClient, n)
	resolved := make([]*containersv1.Mount, 0, len(mounts))

	for _, m := range mounts {

		// Non‑volume mounts are passed through unchanged
		if m.GetVolume() == "" {
			resolved = append(resolved, m)
			continue
		}

		v, err := a.volumeClient.Get(ctx, m.GetVolume())
		if err != nil {
			return fmt.Errorf("get volume %q: %w", m.GetVolume(), err)
		}

		// Pick driver type based on volume.Config.*
		driverType := GetDriverType(v)
		driver, ok := drivers[driverType]
		if !ok {
			return fmt.Errorf("driver %s not configured for node %s",
				driverType.String(), n.GetMeta().GetName())
		}

		// Ensure we have a local Volume instance and it is mounted
		localVol, err := a.ensureLocalVolume(ctx, driver, m.GetVolume())
		if err != nil {
			return fmt.Errorf("prepare volume %q: %w", v.GetMeta().GetName(), err)
		}

		// Create a copy of the mount with resolved source
		nm := proto.Clone(m).(*containersv1.Mount)
		nm.Source = localVol.Location() // what containerd will bind‑mount
		resolved = append(resolved, nm)

		// Update status on the volume
		_ = a.volumeClient.Status().Update(
			ctx,
			m.GetVolume(),
			&volumesv1.Status{
				Controllers: map[string]*volumesv1.ControllerStatus{
					nodeName: {
						Phase:    wrapperspb.String(consts.PHASEATTACHED),
						Ready:    wrapperspb.Bool(true),
						Location: wrapperspb.String(localVol.Location()),
					},
				},
			},
			"controllers",
		)

	}

	// Overwrite container mounts with resolved slice
	ctr.Config.Mounts = resolved
	return nil
}

// Detach implements Attacher interface
func (a *DefaultAttacher) Detach(ctx context.Context, n *nodesv1.Node, ctr *containersv1.Container) error {
	mounts := ctr.GetConfig().GetMounts()
	if len(mounts) == 0 {
		return nil
	}

	// Get usable drivers from the node
	drivers := NodeVolumeDrivers(a.volumeClient, n)
	nodeName := n.GetMeta().GetName()

	for _, m := range mounts {

		if m.GetVolume() == "" {
			continue
		}

		v, err := a.volumeClient.Get(ctx, m.GetVolume())
		if err != nil {
			return err
		}

		driverType := GetDriverType(v)
		driver, ok := drivers[driverType]
		if !ok {
			return fmt.Errorf("driver %s not configured for node %s",
				driverType.String(), n.GetMeta().GetName())
		}

		// Delete volume, and remove content
		err = driver.Delete(ctx, m.GetVolume())
		if err != nil {
			return err
		}

		// Update status on the volume
		_ = a.volumeClient.Status().Update(
			ctx,
			m.GetVolume(),
			&volumesv1.Status{
				Controllers: map[string]*volumesv1.ControllerStatus{
					nodeName: {
						Phase: wrapperspb.String(consts.PHASEPDETACHED),
						Ready: wrapperspb.Bool(false),
					},
				},
			},
			"controllers",
		)

	}

	return nil
}

// ensureLocalVolume gets the volume using the provided driver.
// This method creates the volume using the driver if the volume doesn't exist.
func (a *DefaultAttacher) ensureLocalVolume(ctx context.Context, d Driver, volName string) (Volume, error) {
	vol, err := d.Get(ctx, volName)
	if err != nil {
		if os.IsNotExist(err) {

			// Not present locally → let driver provision it
			vol, err = d.Create(ctx, volName)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	return vol, nil
}
