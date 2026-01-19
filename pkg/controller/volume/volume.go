package volumecontroller

import (
	"context"
	"errors"

	eventsv1 "github.com/amimof/voiyd/api/services/events/v1"
	nodesv1 "github.com/amimof/voiyd/api/services/nodes/v1"
	volumesv1 "github.com/amimof/voiyd/api/services/volumes/v1"
	"github.com/amimof/voiyd/pkg/client"
	"github.com/amimof/voiyd/pkg/consts"
	"github.com/amimof/voiyd/pkg/events"
	"github.com/amimof/voiyd/pkg/labels"
	"github.com/amimof/voiyd/pkg/logger"
	"github.com/amimof/voiyd/pkg/volume"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type NewOption func(c *Controller)

func WithConfig(n *nodesv1.Node) NewOption {
	return func(c *Controller) {
		c.node = n
	}
}

func WithVolumeDrivers(drivers map[volume.DriverType]volume.Driver) NewOption {
	return func(c *Controller) {
		c.volDrivers = drivers
	}
}

func WithHostLocalVolumeDriver(m volume.Driver) NewOption {
	return func(c *Controller) {
		c.volDrivers[volume.DriverTypeHostLocal] = m
	}
}

func WithLogger(l logger.Logger) NewOption {
	return func(c *Controller) {
		c.logger = l
	}
}

var ErrVolumeDriverNotFound = errors.New("volume driver not found")

type Controller struct {
	node       *nodesv1.Node
	logger     logger.Logger
	clientset  *client.ClientSet
	volDrivers map[volume.DriverType]volume.Driver
}

func (c *Controller) handleNodeSelector(h events.VolumeHandlerFunc) events.VolumeHandlerFunc {
	return func(ctx context.Context, volume *volumesv1.Volume) error {
		nodeID := c.node.GetMeta().GetName()

		if !c.isNodeSelected(ctx, nodeID, volume) {
			c.logger.Debug("discarded due to label mismatch", "task", volume.GetMeta().GetName())
			return nil
		}

		err := h(ctx, volume)
		if err != nil {
			return err
		}

		return nil
	}
}

func (vc *Controller) handleVolume(h events.VolumeHandlerFunc) events.HandlerFunc {
	return func(ctx context.Context, ev *eventsv1.Event) error {
		var volume volumesv1.Volume
		err := ev.GetObject().UnmarshalTo(&volume)
		if err != nil {
			return err
		}

		err = h(ctx, &volume)
		if err != nil {
			return err
		}

		return nil
	}
}

func (vc *Controller) handle(h events.HandlerFunc) events.HandlerFunc {
	return func(ctx context.Context, ev *eventsv1.Event) error {
		err := h(ctx, ev)
		if err != nil {
			vc.logger.Error("handler returned error", "event", ev.GetType().String(), "error", err)
			return err
		}
		return nil
	}
}

func (vc *Controller) isNodeSelected(ctx context.Context, nodeID string, volume *volumesv1.Volume) bool {
	node, err := vc.clientset.NodeV1().Get(ctx, nodeID)
	if err != nil {
		return false
	}
	return labels.NewCompositeSelectorFromMap(volume.GetConfig().GetNodeSelector()).Matches(node.GetMeta().GetLabels())
}

// Helper that sets the status on the volume and passes through the error
func (vc *Controller) setControllerStatus(ctx context.Context, volumeID, errConst string, err error) error {
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
func (vc *Controller) getVolumeDriver(vol *volumesv1.Volume) (volume.Driver, error) {
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

func (vc *Controller) Run(ctx context.Context) {
	// Subscribe to events
	ctx = metadata.AppendToOutgoingContext(ctx, "voiyd_controller_name", "volume")

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
	)

	// Setup Node Handlers
	vc.clientset.EventV1().On(events.VolumeCreate, vc.handle(vc.handleVolume(vc.handleNodeSelector(vc.onVolumeCreate))))
	vc.clientset.EventV1().On(events.VolumeDelete, vc.handle(vc.handleVolume(vc.onVolumeDelete)))
	vc.clientset.EventV1().On(events.NodeJoin, vc.handle(vc.onNodeJoin))

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

func (vc *Controller) onNodeJoin(ctx context.Context, ev *eventsv1.Event) error {
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

func (vc *Controller) onVolumeCreate(ctx context.Context, volume *volumesv1.Volume) error {
	id := volume.GetMeta().GetName()
	nodeName := vc.node.GetMeta().GetName()

	// Get the driver the spec asks for from the controller
	volDriver, err := vc.getVolumeDriver(volume)
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

func (vc *Controller) onVolumeDelete(ctx context.Context, volume *volumesv1.Volume) error {
	id := volume.GetMeta().GetName()
	nodeName := vc.node.GetMeta().GetName()

	// Get the driver the spec asks for from the controller
	volDriver, err := vc.getVolumeDriver(volume)
	if err != nil {
		return vc.setControllerStatus(ctx, id, consts.ERRDELETE, err)
	}

	localVol, err := volDriver.Get(ctx, volume.GetMeta().GetName())
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
func (vc *Controller) Reconcile(ctx context.Context) error {
	vc.logger.Info("reconciling volumes in runtime")

	// Get tasks.from the server. Ultimately we want these to match with our runtime
	vlist, err := vc.clientset.VolumeV1().List(ctx)
	if err != nil {
		return err
	}

	nodeName := vc.node.GetMeta().GetName()

	for _, vol := range vlist {

		// Reconcile host-local volumes. Template volumes will be created and attached
		// dynamically on task startup
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

func New(c *client.ClientSet, n *nodesv1.Node, opts ...NewOption) *Controller {
	m := &Controller{
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
