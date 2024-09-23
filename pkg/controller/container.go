package controller

import (
	"context"
	"fmt"
	"log"
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

type ContainerEventHandlerFuncs struct {
	OnContainerCreate func(obj *events.Event)
	OnContainerDelete func(obj *events.Event)
	OnContainerStart  func(obj *events.Event)
	OnContainerKill   func(obj *events.Event)
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
	evt := make(chan *events.Event)
	errChan := make(chan error)

	go func() {
		for {
			select {
			case ev := <-evt:
				c.logger.Info("received event", "id", ev.GetId(), "type", ev.GetType().String())
				handleEventEvent(c.handlers, ev, c.logger)
			case err := <-errChan:
				c.logger.Error("recevied error on channel", "error", err)
				return
			case <-stopCh:
				c.logger.Info("done watching, closing controller")
				ctx.Done()
				return
			}
		}
	}()

	for {
		if err := c.clientset.EventV1().Subscribe(ctx, evt, errChan); err != nil {
			c.logger.Error("error occured during subscribe", "error", err)
		}

		c.logger.Info("attempting to re-subscribe to event server")
		time.Sleep(5 * time.Second)

	}
}

func (c *ContainerController) Reconcile(ctx context.Context) error {
	return nil
}

func handleEventEvent(funcs *ContainerEventHandlerFuncs, ev *events.Event, l logger.Logger) {
	if ev == nil {
		return
	}
	t := ev.Type
	switch t {
	case events.EventType_ContainerCreate:
		funcs.OnContainerCreate(ev)
	case events.EventType_ContainerDelete:
		funcs.OnContainerDelete(ev)
	case events.EventType_ContainerStart:
		funcs.OnContainerStart(ev)
	case events.EventType_ContainerKill:
		funcs.OnContainerKill(ev)
	default:
		l.Warn("Container handler not implemented for event", "type", fmt.Sprintf("%s", t))
	}
}

func (c *ContainerController) handleError(id string, evtType events.EventType, msg string) {
	ctx := context.Background()
	err := c.clientset.ContainerV1().SetContainerHealth(ctx, id, "unhealthy")
	if err != nil {
		c.logger.Error("error setting condition on container", "id", id, "error", err)
	}
	err = c.clientset.ContainerV1().AddContainerEvent(ctx, id, &containers.Event{
		Description: msg,
		Type:        evtType,
	})
	if err != nil {
		c.logger.Error("error adding container event", "id", id, "error", err)
	}
}

func (c *ContainerController) onContainerCreate(obj *events.Event) {
	ctx := context.Background()
	cont, err := c.clientset.ContainerV1().GetContainer(ctx, obj.Id)
	if cont == nil {
		c.logger.Error("container not found", "id", obj.Id)
		return
	}
	if err != nil {
		c.logger.Error("error getting container", "error", err, "id", obj.Id)
		return
	}
	err = c.runtime.Create(ctx, cont)
	if err != nil {
		c.handleError(cont.GetName(), events.EventType_ContainerCreate, fmt.Sprintf("error creating container %s: %s", cont.GetName(), err))
		return
	}
	log.Printf("successfully created container: %s", cont.Name)
}

func (c *ContainerController) onContainerDelete(obj *events.Event) {
	ctx := context.Background()
	err := c.runtime.Delete(ctx, obj.Id)
	if err != nil {
		log.Printf("error deleting container %s from runtime: %s", obj.GetId(), err)
		return
	}
	log.Printf("successfully deleted container %s", obj.Id)
}

func (c *ContainerController) onContainerStart(obj *events.Event) {
	ctx := context.Background()
	ctr, err := c.clientset.ContainerV1().GetContainer(ctx, obj.Id)
	if err != nil {
		log.Printf("error getting container %s", err)
	}
	err = c.runtime.Start(ctx, ctr)
	if err != nil {
		c.handleError(obj.GetId(), events.EventType_ContainerStart, fmt.Sprintf("error starting container %s: %s", ctr.GetName(), err))
		return
	}
	log.Printf("successfully started container %s", obj.Id)
}

func (c *ContainerController) onContainerStop(obj *events.Event) {
	ctx := context.Background()
	ctr, err := c.clientset.ContainerV1().GetContainer(ctx, obj.Id)
	if err != nil {
		log.Printf("error getting container %s", err)
	}
	err = c.runtime.Kill(ctx, ctr)
	if err != nil {
		c.handleError(obj.GetId(), events.EventType_ContainerKill, fmt.Sprintf("error stopping container %s: %s", obj.GetId(), err))
		return
	}
	log.Printf("successfully killed container %s", obj.Id)
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
		OnContainerKill:   eh.onContainerStop,
	}

	for _, opt := range opts {
		opt(eh)
	}

	return eh
}
