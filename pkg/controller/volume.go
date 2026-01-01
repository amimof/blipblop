package controller

import (
	"context"
	"errors"

	eventsv1 "github.com/amimof/blipblop/api/services/events/v1"
	nodesv1 "github.com/amimof/blipblop/api/services/nodes/v1"
	volumesv1 "github.com/amimof/blipblop/api/services/volumes/v1"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/amimof/blipblop/pkg/consts"
	"github.com/amimof/blipblop/pkg/events"
	"github.com/amimof/blipblop/pkg/logger"
	"github.com/amimof/blipblop/pkg/volume"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type NewVolumeControllerOption func(c *VolumeController)

func WithVolumeControllerNodeConfig(n *nodesv1.Node) NewVolumeControllerOption {
	return func(c *VolumeController) {
		c.node = n
	}
}

func WithVolumeDrivers(drivers map[volume.DriverType]volume.Driver) NewVolumeControllerOption {
	return func(c *VolumeController) {
		c.volDrivers = drivers
	}
}

func WithHostLocalVolumeDriver(m volume.Driver) NewVolumeControllerOption {
	return func(c *VolumeController) {
		c.volDrivers[volume.DriverTypeHostLocal] = m
	}
}

func WithVolumeControllerLogger(l logger.Logger) NewVolumeControllerOption {
	return func(c *VolumeController) {
		c.logger = l
	}
}

var ErrVolumeDriverNotFound = errors.New("volume driver not found")

type VolumeController struct {
	node       *nodesv1.Node
	logger     logger.Logger
	clientset  *client.ClientSet
	volDrivers map[volume.DriverType]volume.Driver
}

func (vc *VolumeController) handleErrors(h events.HandlerFunc) events.HandlerFunc {
	return func(ctx context.Context, ev *eventsv1.Event) error {
		err := h(ctx, ev)
		if err != nil {
			vc.logger.Error("handler returned error", "event", ev.GetType().String(), "error", err)
			return err
		}
		return nil
	}
}

// Helper that sets the status on the volume and passes through the error
func (vc *VolumeController) setControllerStatus(ctx context.Context, volumeID, errConst string, err error) error {
	nodeName := vc.node.GetMeta().GetName()
	statusErr := vc.clientset.VolumeV1().Status().Update(
		ctx,
		volumeID,
		&volumesv1.Status{
			Controllers: map[string]*volumesv1.ControllerStatus{
				nodeName: {
					Phase: wrapperspb.String(errConst),
					Ready: wrapperspb.Bool(false),
				},
			},
		},
		"controllers",
	)
	if statusErr != nil {
		vc.logger.Warn("couldn't update volume status", "reason", statusErr, "volume", volumeID)
	}
	return err
}

// getVolumeDriver picks a volume driver on the controller based on what is defined in the Volume spec.
// If the volume controller does not have the driver the Volume asks for then the return value will be nil and error will be ErrNoVolumeDriver.
func (vc *VolumeController) getVolumeDriver(vol *volumesv1.Volume) (volume.Driver, error) {
	cfg := vol.GetConfig()
	if cfg.GetHostLocal() != nil {

		volDriver, ok := vc.volDrivers[volume.DriverTypeHostLocal]
		if !ok {
			return nil, ErrVolumeDriverNotFound
		}
		return volDriver, nil
	}
	if cfg.GetTemplate() != nil {

		volDriver, ok := vc.volDrivers[volume.DriverTypeTemplate]
		if !ok {
			return nil, ErrVolumeDriverNotFound
		}
		return volDriver, nil
	}
	return nil, ErrVolumeDriverNotFound
}

func (vc *VolumeController) Run(ctx context.Context) {
	// Subscribe to events
	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_controller_name", "volume")

	err := vc.Reconcile(ctx)
	if err != nil {
		vc.logger.Error("error reconciling volume controller", "error", err)
		return
	}

	evt, errCh := vc.clientset.EventV1().Subscribe(
		ctx,
		events.VolumeCreate,
		events.VolumeDelete,
		events.VolumeUpdate,
		events.NodeJoin,
		events.NodeConnect,
		events.NodeForget,
	)

	// Setup Node Handlers
	vc.clientset.EventV1().On(events.VolumeCreate, vc.handleErrors(vc.onVolumeCreate))
	vc.clientset.EventV1().On(events.VolumeDelete, vc.handleErrors(vc.onVolumeDelete))
	vc.clientset.EventV1().On(events.NodeJoin, vc.handleErrors(vc.onNodeJoin))
	// vc.clientset.EventV1().On(events.NodeConnect, vc.handleErrors(vc.onNodeConnect))
	// vc.clientset.EventV1().On(events.NodeForget, vc.handleErrors(vc.onNodeForget))

	go func() {
		for e := range evt {
			vc.logger.Info("volume controller got event", "event", e.GetType().String(), "objectID", e.GetObjectId())
		}
	}()

	// Handle errors
	for {
		select {
		case <-ctx.Done():
			return
		case e, ok := <-errCh:
			if !ok {
				errCh = nil
				continue
			}
			if e != nil {
				vc.logger.Error("received error on channel", "error", e)
			}
		}
	}
}

func (vc *VolumeController) onNodeJoin(ctx context.Context, ev *eventsv1.Event) error {
	nodeSpec := &nodesv1.Node{}
	err := ev.GetObject().UnmarshalTo(nodeSpec)
	if err != nil {
		return err
	}

	nodeName := vc.node.GetMeta().GetName()
	joinedNode := nodeSpec.GetMeta().GetName()

	if nodeName == joinedNode {
		vc.logger.Debug("volume controller self-aware", "node", nodeName)
	}

	volumes, err := vc.clientset.VolumeV1().List(ctx)
	if err != nil {
		return err
	}

	for _, vol := range volumes {
		id := vol.GetMeta().GetName()
		volDriver, err := vc.getVolumeDriver(vol)
		if err != nil {
			return vc.setControllerStatus(ctx, id, consts.ERRPROVISIONING, err)
		}

		_, err = volDriver.Create(ctx, id)
		if err != nil {
			return vc.setControllerStatus(ctx, id, consts.ERRPROVISIONING, err)
		}
	}

	return nil
}

func (vc *VolumeController) onVolumeCreate(ctx context.Context, ev *eventsv1.Event) error {
	volSpec := &volumesv1.Volume{}
	err := ev.GetObject().UnmarshalTo(volSpec)
	if err != nil {
		return err
	}

	id := volSpec.GetMeta().GetName()
	nodeName := vc.node.GetMeta().GetName()

	// Get the driver the spec asks for from the controller
	volDriver, err := vc.getVolumeDriver(volSpec)
	if err != nil {
		return vc.setControllerStatus(ctx, id, consts.ERRPROVISIONING, err)
	}

	vol, err := volDriver.Create(ctx, id)
	if err != nil {
		return vc.setControllerStatus(ctx, id, consts.ERRPROVISIONING, err)
	}

	vc.logger.Debug("created volume", "id", vol.ID(), "location", vol.Location())

	return vc.clientset.VolumeV1().Status().Update(
		ctx,
		string(vol.ID()),
		&volumesv1.Status{
			Controllers: map[string]*volumesv1.ControllerStatus{
				nodeName: {
					Phase:    wrapperspb.String(consts.PHASEPROVISIONED),
					Location: wrapperspb.String(vol.Location()),
					Ready:    wrapperspb.Bool(true),
				},
			},
		},
		"controllers",
	)
}

func (vc *VolumeController) onVolumeDelete(ctx context.Context, ev *eventsv1.Event) error {
	volSpec := &volumesv1.Volume{}
	err := ev.GetObject().UnmarshalTo(volSpec)
	if err != nil {
		return err
	}

	id := volSpec.GetMeta().GetName()
	nodeName := vc.node.GetMeta().GetName()

	// Get the driver the spec asks for from the controller
	volDriver, err := vc.getVolumeDriver(volSpec)
	if err != nil {
		return vc.setControllerStatus(ctx, id, consts.ERRDELETE, err)
	}

	localVol, err := volDriver.Get(ctx, volSpec.GetMeta().GetName())
	if err != nil {
		return vc.setControllerStatus(ctx, id, consts.ERRDELETE, err)
	}

	if err := volDriver.Delete(ctx, id); err != nil {
		return vc.setControllerStatus(ctx, id, consts.ERRDELETE, err)
	}

	vc.logger.Debug("deleted host-local volume", "id", localVol.ID(), "location", localVol.Location())

	// Update status once deleted
	// TODO: This will probably always fail since updating a deleted volume is pointless
	return vc.clientset.VolumeV1().Status().Update(
		ctx,
		id,
		&volumesv1.Status{
			Controllers: map[string]*volumesv1.ControllerStatus{
				nodeName: {},
			},
		},
		"controllers",
	)
}

// Reconcile implements Controller.
func (vc *VolumeController) Reconcile(ctx context.Context) error {
	vc.logger.Info("reconciling volumes in runtime")

	// Get containers from the server. Ultimately we want these to match with our runtime
	vlist, err := vc.clientset.VolumeV1().List(ctx)
	if err != nil {
		return err
	}

	nodeName := vc.node.GetMeta().GetName()

	for _, vol := range vlist {

		// Reconcile host-local volumes. Template volumes will be created and attached
		// dynamically on container startup
		driverType := volume.GetDriverType(vol)
		if driverType == volume.DriverTypeHostLocal {
			id := vol.GetMeta().GetName()
			driver, err := vc.getVolumeDriver(vol)
			if err != nil {
				return vc.setControllerStatus(ctx, id, consts.ERRPROVISIONING, err)
			}

			driverVol, err := driver.Create(ctx, vol.GetMeta().GetName())
			if err != nil {
				return vc.setControllerStatus(ctx, id, consts.ERRPROVISIONING, err)
			}

			vc.logger.Debug("created volume on reconcile", "volume", driverVol.ID(), "location", driverVol.Location())

			_ = vc.clientset.VolumeV1().Status().Update(
				ctx,
				string(driverVol.ID()),
				&volumesv1.Status{
					Controllers: map[string]*volumesv1.ControllerStatus{
						nodeName: {
							Phase:    wrapperspb.String(consts.PHASEPROVISIONED),
							Location: wrapperspb.String(driverVol.Location()),
							Ready:    wrapperspb.Bool(true),
						},
					},
				},
				"controllers",
			)
		}
	}

	return nil
}

func NewVolumeController(c *client.ClientSet, n *nodesv1.Node, opts ...NewVolumeControllerOption) *VolumeController {
	m := &VolumeController{
		node:       n,
		clientset:  c,
		logger:     logger.ConsoleLogger{},
		volDrivers: make(map[volume.DriverType]volume.Driver),
	}
	for _, opt := range opts {
		opt(m)
	}

	return m
}
