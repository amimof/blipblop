package events

import (
	"log"
	"sync"

	eventsv1 "github.com/amimof/blipblop/api/services/events/v1"
)

type EventInformer struct {
	handlers ResourceEventHandlerFuncs
	mu       sync.RWMutex
}

func (e *EventInformer) AddHandlers(funcs ResourceEventHandlerFuncs) {
	e.handlers = funcs
}

func (i *EventInformer) Run(eventChan chan *eventsv1.Event) {
	for e := range eventChan {
		switch e.Operation {
		case eventsv1.Operation_Create:
			if i.handlers.OnCreate != nil {
				i.handlers.OnCreate(e)
			}
		case eventsv1.Operation_Update:
			if i.handlers.OnUpdate != nil {
				i.handlers.OnUpdate(e)
			}
		case eventsv1.Operation_Delete:
			if i.handlers.OnDelete != nil {
				i.handlers.OnDelete(e)
			}
		default:
			log.Printf("informer received unimplemented event type %s", e.GetType().String())
		}
	}
}

func NewEventInformer() *EventInformer {
	return &EventInformer{}
}
