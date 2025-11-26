package controller

import (
	"context"
	"errors"

	eventsv1 "github.com/amimof/blipblop/api/services/events/v1"
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

func WithVolumeDrivers(drivers map[volume.VolumeType]volume.Driver) NewVolumeControllerOption {
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
	logger    logger.Logger
	clientset *client.ClientSet
	// volManager volume.Driver
	volDrivers map[volume.VolumeType]volume.Driver
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
	return nil, ErrVolumeDriverNotFound
}

func (vc *VolumeController) Run(ctx context.Context) {
	// Subscribe to events
	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_controller_name", "volume")

	evt, errCh := vc.clientset.EventV1().Subscribe(
		ctx,
		events.VolumeCreate,
		events.VolumeDelete,
		events.VolumeUpdate,
	)

	// Setup Node Handlers
	vc.clientset.EventV1().On(events.VolumeCreate, vc.handleErrors(vc.onVolumeCreate))
	vc.clientset.EventV1().On(events.VolumeDelete, vc.onVolumeDelete)

	go func() {
		for e := range evt {
			vc.logger.Info("Got event", "event", e.GetType().String(), "objectID", e.GetObjectId())
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

func (vc *VolumeController) onVolumeCreate(ctx context.Context, ev *eventsv1.Event) error {
	volSpec := &volumesv1.Volume{}
	err := ev.GetObject().UnmarshalTo(volSpec)
	if err != nil {
		return err
	}

	// Get the driver the spec asks for from the controller
	volDriver, err := vc.getVolumeDriver(volSpec)
	if err != nil {
		return err
	}

	hostLocalVolume := volSpec.GetConfig().GetHostLocal()
	id := hostLocalVolume.GetName()
	vol, err := volDriver.Create(ctx, id)
	if err != nil {
		_ = vc.clientset.VolumeV1().Status().Update(
			ctx,
			id,
			&volumesv1.Status{
				Phase: wrapperspb.String(consts.ERRPROVISIONING),
			},
			"phase",
		)
		return err
	}

	vc.logger.Debug("created host-local volume", "id", vol.ID(), "location", vol.Location())

	err = vc.clientset.VolumeV1().Status().Update(
		ctx,
		string(vol.ID()),
		&volumesv1.Status{
			Location: wrapperspb.String(vol.Location()),
			Phase:    wrapperspb.String(consts.PHASEPROVISIONED),
		},
		"phase", "location",
	)
	if err != nil {
		vc.logger.Error("error setting status on volume", "error", err)
	}

	return err
}

func (vc *VolumeController) onVolumeDelete(ctx context.Context, ev *eventsv1.Event) error {
	volSpec := &volumesv1.Volume{}
	err := ev.GetObject().UnmarshalTo(volSpec)
	if err != nil {
		return err
	}

	// Get the driver the spec asks for from the controller
	volDriver, err := vc.getVolumeDriver(volSpec)
	if err != nil {
		return err
	}

	hostLocalVolume := volSpec.GetConfig().GetHostLocal()
	id := hostLocalVolume.GetName()
	if err := volDriver.Delete(ctx, id); err != nil {
		_ = vc.clientset.VolumeV1().Status().Update(
			ctx,
			id,
			&volumesv1.Status{
				Phase: wrapperspb.String(consts.ERRDELETE),
			},
			"phase",
		)
		return err
	}

	return nil
}

// Reconcile implements Controller.
func (vc *VolumeController) Reconcile(context.Context) error {
	panic("unimplemented")
}

func NewVolumeController(c *client.ClientSet, opts ...NewVolumeControllerOption) *VolumeController {
	m := &VolumeController{
		clientset:  c,
		logger:     logger.ConsoleLogger{},
		volDrivers: make(map[volume.VolumeType]volume.Driver),
	}
	for _, opt := range opts {
		opt(m)
	}

	return m
}
