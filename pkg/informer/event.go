package informer

import (
	"context"
	"log"

	"github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/pkg/client"
)

type EventInformer struct {
	handlers *EventHandlerFuncs
	client   *client.ClientSet
}

type EventHandlerFuncs struct {
	OnContainerCreate func(obj *events.Event)
	OnContainerDelete func(obj *events.Event)
	OnContainerStart  func(obj *events.Event)
	OnContainerStop   func(obj *events.Event)
}

func (i *EventInformer) AddHandler(h *EventHandlerFuncs) {
	i.handlers = h
}

func (i *EventInformer) Watch(ctx context.Context, stopCh <-chan struct{}) {
	evc, errc := i.client.EventV1().Subscribe(ctx)
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
		case <-ctx.Done():
			log.Println("Done, closing client")
			i.client.Close()
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
	case events.EventType_ContainerStart:
		h.OnContainerStart(ev)
	case events.EventType_ContainerStop:
		h.OnContainerStop(ev)
	default:
		log.Printf("Handler not implemented for event type %s", t)
	}
}

func handleEventError(err error) {
	// if err != nil && err != io.EOF {
	// 	log.Printf("error occurred handling error %s", err.Error())
	// }
}

func NewEventInformer(client *client.ClientSet) *EventInformer {
	return &EventInformer{
		client: client,
	}
}
