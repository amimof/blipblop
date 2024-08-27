package controller

import (
	"context"
	"fmt"
	"log"

	"github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/amimof/blipblop/pkg/runtime"
)

type ContainerController struct {
	clientset *client.ClientSet
	// runtime   *client.RuntimeClient
	runtime  runtime.Runtime
	handlers *ContainerEventHandlerFuncs
}

type ContainerEventHandlerFuncs struct {
	OnContainerCreate func(obj *events.Event)
	OnContainerDelete func(obj *events.Event)
	OnContainerStart  func(obj *events.Event)
	OnContainerKill   func(obj *events.Event)
}

func (i *ContainerController) AddHandler(h *ContainerEventHandlerFuncs) {
	i.handlers = h
}

func (i *ContainerController) Run(ctx context.Context, stopCh <-chan struct{}) {
	evc, errc := i.clientset.EventV1().Subscribe(ctx)
	for {
		select {
		case ev := <-evc:
			handleEventEvent(i.handlers, ev)
		case err := <-errc:
			log.Printf("error received on channel: %s", err)
		case <-stopCh:
			ctx.Done()
			log.Println("Done watching event informer")
			return
		case <-ctx.Done():
			log.Println("Done, closing client")
			i.clientset.Close()
			return
		}
	}
}

func handleEventEvent(h *ContainerEventHandlerFuncs, ev *events.Event) {
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
	default:
		log.Printf("Handler not implemented for event type %s", t)
	}
}

func (r *ContainerController) handleError(e error, id, msg string) {
	log.Printf("%s", msg)
	ctx := context.Background()
	err := r.clientset.ContainerV1().SetContainerCondition(ctx, id, e.Error())
	if err != nil {
		log.Printf("error setting condition for container %s: %s", id, err)
	}
}

func (c *ContainerController) onContainerCreate(obj *events.Event) {
	ctx := context.Background()
	cont, err := c.clientset.ContainerV1().GetContainer(ctx, obj.Id)
	if cont == nil {
		log.Printf("container %s not found", obj.Id)
		return
	}
	if err != nil {
		log.Printf("error occurred: %s", err.Error())
		return
	}
	err = c.runtime.Create(ctx, cont)
	if err != nil {
		c.handleError(err, cont.GetName(), fmt.Sprintf("error creating container %s: %s", cont.GetName(), err))
		return
	}
	log.Printf("successfully created container: %s", cont.Name)
}

func (c *ContainerController) onContainerDelete(obj *events.Event) {
	ctx := context.Background()
	err := c.runtime.Delete(ctx, obj.Id)
	if err != nil {
		c.handleError(err, obj.GetId(), fmt.Sprintf("error deleting container %s: %s", obj.GetId(), err))
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
	err = c.runtime.Start(ctx, ctr.GetName())
	if err != nil {
		c.handleError(err, obj.GetId(), fmt.Sprintf("error starting container %s: %s", ctr.GetName(), err))
		return
	}
	log.Printf("successfully started container %s", obj.Id)
}

func (c *ContainerController) onContainerStop(obj *events.Event) {
	ctx := context.Background()
	err := c.runtime.Kill(ctx, obj.Id)
	if err != nil {
		c.handleError(err, obj.GetId(), fmt.Sprintf("error stopping container %s: %s", obj.GetId(), err))
		return
	}
	log.Printf("successfully killed container %s", obj.Id)
}

func NewContainerController(cs *client.ClientSet, rt runtime.Runtime) *ContainerController {
	eh := &ContainerController{
		clientset: cs,
		runtime:   rt,
	}

	handlers := &ContainerEventHandlerFuncs{
		OnContainerCreate: eh.onContainerCreate,
		OnContainerDelete: eh.onContainerDelete,
		OnContainerStart:  eh.onContainerStart,
		OnContainerKill:   eh.onContainerStop,
	}

	eh.handlers = handlers

	return eh
}
