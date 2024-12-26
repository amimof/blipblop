package eventsv2

import (
	"context"

	eventsv1 "github.com/amimof/blipblop/api/services/events/v1"
)

type Subscriber interface {
	Subscribe(context.Context, eventsv1.EventType) <-chan *eventsv1.Event
	Unsubscribe(context.Context, eventsv1.EventType) error
}

type Publisher interface {
	Publish(context.Context, eventsv1.EventType, *eventsv1.Event) error
}

type Forwarder interface {
	Forward(context.Context, eventsv1.EventType, *eventsv1.Event) error
}
