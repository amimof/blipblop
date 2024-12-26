package informer

import (
	"context"

	eventsv1 "github.com/amimof/blipblop/api/services/events/v1"
)

var (
	UnimplemetedEventHandlerFunc = func(e *eventsv1.Event) {}
	UnimplemetedErrorHandlerFunc = func(err error) {}
)

type EventInformer interface {
	Run(context.Context, <-chan *eventsv1.Event)
}

type EventHandlerFunc func(*eventsv1.Event) error
