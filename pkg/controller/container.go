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
	OnNodeDelete      func(obj *events.Event)
}

type NewContainerControllerOption func(c *ContainerController)

func WithContainerControllerLogger(l logger.Logger) NewContainerControllerOption {
	return func(c *ContainerController) {
		c.logger = l
	}
}

func (i *ContainerController) AddHandler(h *ContainerEventHandlerFuncs) {
	i.handlers = h
}

func (i *ContainerController) Run(ctx context.Context, stopCh <-chan struct{}) {
	// Setup channels
	evt := make(chan *events.Event)
	errChan := make(chan error)

	go func() {
		for {
			select {
			case ev := <-evt:
				i.logger.Info("received event", "id", ev.GetId(), "type", ev.GetType().String())
				handleEventEvent(i.handlers, ev, i.logger)
			case err := <-errChan:
				i.logger.Error("recevied error on channel", "error", err)
				return
			case <-stopCh:
				i.logger.Info("done watching, closing controller")
				ctx.Done()
				return
			}
		}
	}()

	for {
		if err := i.clientset.EventV1().Subscribe(ctx, evt, errChan); err != nil {
			i.logger.Error("error occured during subscribe", "error", err)
		}

		i.logger.Info("attempting to re-subscribe to event server")
		time.Sleep(5 * time.Second)

	}
}

func handleEventEvent(h *ContainerEventHandlerFuncs, ev *events.Event, l logger.Logger) {
	if ev == nil {
		return
	}
	t := ev.Type
	switch t {
	case events.EventType_ContainerCreate:
		h.OnContainerCreate(ev)
	case events.EventType_ContainerDelete:
		h.OnContainerDelete(ev)
	case events.EventType_ContainerStart:
		h.OnContainerStart(ev)
	case events.EventType_ContainerKill:
		h.OnContainerKill(ev)
	case events.EventType_NodeDelete:
		h.OnNodeDelete(ev)
	default:
		l.Warn("Handler not implemented for event", "type", t)
	}
}

func (r *ContainerController) handleError(id string, evtType events.EventType, msg string) {
	ctx := context.Background()
	err := r.clientset.ContainerV1().SetContainerHealth(ctx, id, "unhealthy")
	if err != nil {
		r.logger.Error("error setting condition on container", "id", id, "error", err)
	}
	err = r.clientset.ContainerV1().AddContainerEvent(ctx, id, &containers.Event{
		Description: msg,
		Type:        evtType,
	})
	if err != nil {
		r.logger.Error("error adding container event", "id", id, "error", err)
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

func (c *ContainerController) onNodeDelete(obj *events.Event) {
	ctx := context.Background()
	err := c.clientset.NodeV1().ForgetNode(ctx, obj.GetId())
	if err != nil {
		log.Printf("error unjoining node %s: %v", obj.GetId(), err)
		return
	}
	log.Printf("successfully unjoined node %s", obj.GetId())
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

	handlers := &ContainerEventHandlerFuncs{
		OnContainerCreate: eh.onContainerCreate,
		OnContainerDelete: eh.onContainerDelete,
		OnContainerStart:  eh.onContainerStart,
		OnContainerKill:   eh.onContainerStop,
		OnNodeDelete:      eh.onNodeDelete,
	}

	eh.handlers = handlers

	for _, opt := range opts {
		opt(eh)
	}

	return eh
}
