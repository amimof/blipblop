package controller

import (
	"context"
	"time"

	"github.com/amimof/blipblop/api/services/containers/v1"
	eventsv1 "github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/amimof/blipblop/pkg/events"
	"github.com/amimof/blipblop/pkg/logger"
	"github.com/amimof/blipblop/pkg/runtime"
)

type ContainerController struct {
	clientset *client.ClientSet
	runtime   runtime.Runtime
	logger    logger.Logger
}

type NewContainerControllerOption func(c *ContainerController)

func WithContainerControllerLogger(l logger.Logger) NewContainerControllerOption {
	return func(c *ContainerController) {
		c.logger = l
	}
}

func (c *ContainerController) Run(ctx context.Context, stopCh <-chan struct{}) {
	// Setup channels
	evt := make(chan *eventsv1.Event, 10)
	errChan := make(chan error, 10)

	// Setup handlers
	handlers := events.ContainerEventHandlerFuncs{
		OnCreate: c.onContainerCreate,
		OnUpdate: c.onContainerUpdate,
		OnDelete: c.onContainerDelete,
		OnStart:  c.onContainerStart,
		OnKill:   c.onContainerKill,
		OnStop:   c.onContainerStop,
	}

	// Run informer
	informer := events.NewContainerEventInformer(handlers)
	go informer.Run(evt)

	// Subscribe with retry
	for {
		select {
		case <-stopCh:
			c.logger.Info("done watching, stopping subscription")
			return
		default:
			if err := c.clientset.EventV1().Subscribe(ctx, evt, errChan); err != nil {
				c.logger.Error("error occured during subscribe", "error", err)
			}

			c.logger.Info("attempting to re-subscribe to event server")
			time.Sleep(5 * time.Second)
		}
	}
}

func (c *ContainerController) Reconcile(ctx context.Context) error {
	return nil
}

func (c *ContainerController) onContainerCreate(obj *eventsv1.Event) error {
	ctx := context.Background()
	id := obj.GetObjectId()
	// Get the container
	ctr, err := c.clientset.ContainerV1().Get(context.Background(), id)
	if err != nil {
		c.logger.Error("couldn't handle event, error getting container", "error", err, "container", id)
		return err
	}
	err = c.runtime.Pull(ctx, ctr)
	if err != nil {
		return err
	}
	err = c.runtime.Run(ctx, ctr)
	if err != nil {
		return err
	}
	return nil
}

func (c *ContainerController) onContainerUpdate(_ *eventsv1.Event) error {
	return nil
}

func (c *ContainerController) onContainerDelete(obj *eventsv1.Event) error {
	ctx := context.Background()
	id := obj.GetObjectId()
	// Get the container
	ctr, err := c.clientset.ContainerV1().Get(context.Background(), id)
	if err != nil {
		c.logger.Error("couldn't handle event, error getting container", "error", err, "container", id)
		return err
	}
	err = c.runtime.Kill(ctx, ctr)
	if err != nil {
		return err
	}
	err = c.runtime.Delete(ctx, ctr)
	if err != nil {
		return err
	}
	return nil
}

func (c *ContainerController) onContainerStart(obj *eventsv1.Event) error {
	ctx := context.Background()
	id := obj.GetObjectId()
	// Get the container
	ctr, err := c.clientset.ContainerV1().Get(context.Background(), id)
	if err != nil {
		c.logger.Error("couldn't handle event, error getting container", "error", err, "container", id)
		return err
	}

	// Delete container if it exists
	_ = c.clientset.ContainerV1().SetTaskStatus(ctx, ctr.GetMeta().GetName(), containers.Phase_Stopping.String(), "")
	if err := c.runtime.Delete(ctx, ctr); err != nil {
		return err
	}

	// Pull image
	_ = c.clientset.ContainerV1().SetTaskStatus(ctx, ctr.GetMeta().GetName(), containers.Phase_Pulling.String(), "")
	err = c.runtime.Pull(ctx, ctr)
	if err != nil {
		return err
	}

	// Run container
	_ = c.clientset.ContainerV1().SetTaskStatus(ctx, ctr.GetMeta().GetName(), containers.Phase_Starting.String(), "")
	err = c.runtime.Run(ctx, ctr)
	if err != nil {
		return err
	}

	return nil
}

func (c *ContainerController) onContainerKill(obj *eventsv1.Event) error {
	ctx := context.Background()
	id := obj.GetObjectId()
	// Get the container
	ctr, err := c.clientset.ContainerV1().Get(context.Background(), id)
	if err != nil {
		c.logger.Error("couldn't handle event, error getting container", "error", err, "container", id)
		return err
	}
	err = c.runtime.Kill(ctx, ctr)
	if err != nil {
		return err
	}
	return nil
}

func (c *ContainerController) onContainerStop(obj *eventsv1.Event) error {
	ctx := context.Background()
	id := obj.GetObjectId()
	// Get the container
	ctr, err := c.clientset.ContainerV1().Get(context.Background(), id)
	if err != nil {
		c.logger.Error("couldn't handle event, error getting container", "error", err, "container", id)
		return err
	}
	err = c.runtime.Stop(ctx, ctr)
	if err != nil {
		return err
	}
	return nil
}

func NewContainerController(cs *client.ClientSet, rt runtime.Runtime, opts ...NewContainerControllerOption) *ContainerController {
	eh := &ContainerController{
		clientset: cs,
		runtime:   rt,
		logger:    logger.ConsoleLogger{},
	}

	for _, opt := range opts {
		opt(eh)
	}

	return eh
}
