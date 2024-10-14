package controller

import (
	"context"
	"time"

	"github.com/amimof/blipblop/api/services/containers/v1"
	"github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/amimof/blipblop/pkg/logger"
	"github.com/amimof/blipblop/pkg/runtime"
)

type ContainerController struct {
	clientset *client.ClientSet
	// runtime   *client.RuntimeClient
	runtime  runtime.Runtime
	handlers *ContainerEventHandlerFuncs
	logger   logger.Logger
}

type ContainerEventHandlerFunc func(obj *events.Event, ctr *containers.Container) error

type ContainerEventHandlerFuncs struct {
	OnContainerCreate ContainerEventHandlerFunc
	OnContainerDelete ContainerEventHandlerFunc
	OnContainerStart  ContainerEventHandlerFunc
	OnContainerKill   ContainerEventHandlerFunc
	OnContainerStop   ContainerEventHandlerFunc
}

type NewContainerControllerOption func(c *ContainerController)

func WithContainerControllerLogger(l logger.Logger) NewContainerControllerOption {
	return func(c *ContainerController) {
		c.logger = l
	}
}

func (c *ContainerController) AddHandler(h *ContainerEventHandlerFuncs) {
	c.handlers = h
}

func (c *ContainerController) Run(ctx context.Context, stopCh <-chan struct{}) {
	// Setup channels
	evt := make(chan *events.Event, 10)
	errChan := make(chan error, 10)

	go func() {
		for {
			select {
			case ev := <-evt:
				c.logger.Debug("received event", "id", ev.GetMeta().GetName(), "type", ev.GetType().String(), "clientId", ev.GetClientId())
				c.handleEventEvent(c.handlers, ev, c.logger)
				continue
			case err := <-errChan:
				c.logger.Error("recevied error on channel", "error", err)
				return
			case <-stopCh:
				c.logger.Info("done watching, closing controller")
				return
			}
		}
	}()

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

// TODO: This controller will also receive events for objects that are not container-related (sucha as nodes).
// Need to implement logic to only handle container events.
func (c *ContainerController) handleEventEvent(funcs *ContainerEventHandlerFuncs, ev *events.Event, l logger.Logger) {
	ctx := context.Background()

	// Ignore empty events
	if ev == nil {
		return
	}
	if ev.GetObjectId() == "" {
		return
	}

	// Get the container
	ctr, err := c.clientset.ContainerV1().Get(context.Background(), ev.GetObjectId())
	if err != nil {
		c.logger.Error("couldn't handle event, error getting container", "error", err, "objectId", ev.GetObjectId())
		return
	}

	// Run handlers
	t := ev.Type
	switch t {
	case events.EventType_ContainerCreate:
		_ = c.clientset.ContainerV1().SetTaskStatus(ctx, ctr.GetMeta().GetName(), containers.Phase_Starting.String(), "")
		if err := funcs.OnContainerCreate(ev, ctr); err != nil {
			_ = c.clientset.ContainerV1().SetTaskStatus(ctx, ctr.GetMeta().GetName(), containers.Phase_Error.String(), err.Error())
			c.logger.Error("error calling OnContainerCreate handler", "error", err)
		}
	case events.EventType_ContainerDelete:
		_ = c.clientset.ContainerV1().SetTaskStatus(ctx, ctr.GetMeta().GetName(), containers.Phase_Deleting.String(), "")
		if err := funcs.OnContainerDelete(ev, ctr); err != nil {
			_ = c.clientset.ContainerV1().SetTaskStatus(ctx, ctr.GetMeta().GetName(), containers.Phase_Error.String(), err.Error())
			c.logger.Error("error calling OnContainerDelete handler", "error", err)
		}
	case events.EventType_ContainerStart:
		if err := funcs.OnContainerStart(ev, ctr); err != nil {
			c.logger.Error("error calling OnContainerStart handler", "error", err)
		}
	case events.EventType_ContainerKill:
		_ = c.clientset.ContainerV1().SetTaskStatus(ctx, ctr.GetMeta().GetName(), containers.Phase_Stopping.String(), "")
		if err := funcs.OnContainerKill(ev, ctr); err != nil {
			_ = c.clientset.ContainerV1().SetTaskStatus(ctx, ctr.GetMeta().GetName(), containers.Phase_Error.String(), err.Error())
			c.logger.Error("error calling OnContainerKill handler", "error", err)
		}
	default:
		l.Debug("container handler not implemented for event", "type", t.String())
	}
}

// TODO: Also print error to stderr
// func (c *ContainerController) handleError(id string, evtType events.EventType, msg string) {
// 	ctx := context.Background()
// 	err := c.clientset.ContainerV1().SetHealth(ctx, id, "unhealthy")
// 	if err != nil {
// 		c.logger.Error("error setting condition on container", "id", id, "error", err)
// 	}
// 	// err = c.clientset.ContainerV1().AddEvent(ctx, id, &containers.Event{
// 	// 	Description: msg,
// 	// 	Type:        evtType,
// 	// })
// 	// if err != nil {
// 	// 	c.logger.Error("error adding container event", "id", id, "error", err)
// 	// }
// }

func (c *ContainerController) onContainerCreate(obj *events.Event, ctr *containers.Container) error {
	ctx := context.Background()
	err := c.runtime.Pull(ctx, ctr)
	if err != nil {
		return err
	}
	err = c.runtime.Run(ctx, ctr)
	if err != nil {
		return err
	}
	return nil
}

func (c *ContainerController) onContainerDelete(obj *events.Event, ctr *containers.Container) error {
	ctx := context.Background()
	err := c.runtime.Kill(ctx, ctr)
	if err != nil {
		return err
	}
	err = c.runtime.Delete(ctx, ctr)
	if err != nil {
		return err
	}
	return nil
}

func (c *ContainerController) onContainerStart(obj *events.Event, ctr *containers.Container) error {
	ctx := context.Background()
	// Delete container if it exists
	_ = c.clientset.ContainerV1().SetTaskStatus(ctx, ctr.GetMeta().GetName(), containers.Phase_Stopping.String(), "")
	if err := c.runtime.Delete(ctx, ctr); err != nil {
		return err
	}

	// Pull image
	_ = c.clientset.ContainerV1().SetTaskStatus(ctx, ctr.GetMeta().GetName(), containers.Phase_Pulling.String(), "")
	err := c.runtime.Pull(ctx, ctr)
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

func (c *ContainerController) onContainerKill(obj *events.Event, ctr *containers.Container) error {
	ctx := context.Background()
	err := c.runtime.Kill(ctx, ctr)
	if err != nil {
		return err
	}
	return nil
}

func (c *ContainerController) onContainerStop(obj *events.Event, ctr *containers.Container) error {
	ctx := context.Background()
	err := c.runtime.Stop(ctx, ctr)
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

	eh.handlers = &ContainerEventHandlerFuncs{
		OnContainerCreate: eh.onContainerCreate,
		OnContainerDelete: eh.onContainerDelete,
		OnContainerStart:  eh.onContainerStart,
		OnContainerKill:   eh.onContainerKill,
		OnContainerStop:   eh.onContainerStop,
	}

	for _, opt := range opts {
		opt(eh)
	}

	return eh
}
