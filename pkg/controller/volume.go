package controller

import (
	"context"

	eventsv1 "github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/amimof/blipblop/pkg/events"
	"github.com/amimof/blipblop/pkg/logger"
	"github.com/amimof/blipblop/pkg/volume"
	"google.golang.org/grpc/metadata"
)

type NewVolumeControllerOption func(c *VolumeController)

func WithVolumeControllerLogger(l logger.Logger) NewVolumeControllerOption {
	return func(c *VolumeController) {
		c.logger = l
	}
}

func WithVolumeManager(m volume.Manager) NewVolumeControllerOption {
	return func(c *VolumeController) {
		c.volManager = m
	}
}

type VolumeController struct {
	logger     logger.Logger
	clientset  *client.ClientSet
	volManager volume.Manager
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
	vc.clientset.EventV1().On(events.NodeCreate, vc.onVolumeCreate)

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
	return nil
}

// Reconcile implements Controller.
func (vc *VolumeController) Reconcile(context.Context) error {
	panic("unimplemented")
}

func NewVolumeController(c *client.ClientSet, opts ...NewVolumeControllerOption) (*VolumeController, error) {
	m := &VolumeController{
		clientset:  c,
		logger:     logger.ConsoleLogger{},
		volManager: volume.NewHostLocalManager("/var/lib/blipblop/volumes"),
	}
	for _, opt := range opts {
		opt(m)
	}

	return m, nil
}
