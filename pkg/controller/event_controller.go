package controller

import (
	"context"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/amimof/blipblop/api/services/events/v1"
	"github.com/containerd/containerd"
	"log"
)

type EventInformer struct {
	handlers *EventHandlerFuncs
	client   *client.Client
}

type EventHandlerFuncs struct {
	OnContainerCreate func(obj *events.Event)
	OnContainerDelete func(obj *events.Event)
}

type eventController struct {
	client   *client.Client
	runtime  *client.RuntimeClient
	informer *EventInformer
}

func (i *EventInformer) AddHandler(h *EventHandlerFuncs) {
	i.handlers = h
}

func (i *EventInformer) Watch(stopCh <-chan struct{}) {
	ctx := context.Background()
	evc, errc := i.client.Subscribe(ctx)
	for {
		select {
		case ev := <-evc:
			handleEventEvent(i.handlers, ev)
		case err := <-errc:
			handleEventError(err)
		case <-stopCh:
			ctx.Done()
			log.Println("Done watching event informer")
			return
		}
	}
}

func handleEventEvent(h *EventHandlerFuncs, ev *events.Event) {
	if ev == nil {
		return
	}
	t := ev.Type
	switch t {
	case events.EventType_ContainerCreate:
		h.OnContainerCreate(ev)
	case events.EventType_ContainerDelete:
		h.OnContainerDelete(ev)
	default:
		log.Printf("Handler not implemented for event type %s", t)
	}
}

func handleEventError(err error) {
	log.Printf("error occurred handling error %s", err.Error())
}

func (n *eventController) Run(stop <-chan struct{}) {
	go n.informer.Watch(stop)
}

func NewEventInformer(client *client.Client) *EventInformer {
	return &EventInformer{
		client: client,
	}
}

func NewEventController(c *client.Client, cc *containerd.Client) Controller {
	n := &eventController{
		client:  c,
		runtime: client.NewContainerdRuntimeClient(cc, nil),
	}
	informer := NewEventInformer(c)
	informer.AddHandler(&EventHandlerFuncs{
		OnContainerCreate: func(obj *events.Event) {
			log.Println("not implemented: OnContainerCreate")
			cont, err := c.GetContainer(context.Background(), "asd")
			if err != nil {
				log.Printf("error occurred: %s", err.Error())
			}
			log.Printf("Got container: %s", cont.Image)
		},
		OnContainerDelete: func(obj *events.Event) {
			log.Println("not implemented: OnContainerDelete")
		},
	})
	n.informer = informer
	return n
}
