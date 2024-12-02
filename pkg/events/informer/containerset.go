package informer

import (
	"context"

	eventsv1 "github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/pkg/logger"
)

type NewContainerSetEventInformerOption func(s *containerSetEventInformer)

func WithContainerSetEventInformerLogger(l logger.Logger) NewContainerSetEventInformerOption {
	return func(s *containerSetEventInformer) {
		s.logger = l
	}
}

type ContainerSetEventHandlerFuncs struct {
	OnCreate ResourceEventHandlerFunc
	OnUpdate ResourceEventHandlerFunc
	OnDelete ResourceEventHandlerFunc
}
type containerSetEventInformer struct {
	handlers ContainerSetEventHandlerFuncs
	logger   logger.Logger
}

func (i *containerSetEventInformer) Run(ctx context.Context, eventChan <-chan *eventsv1.Event) {
	for e := range eventChan {
		switch e.Type {
		case eventsv1.EventType_ContainerSetCreate:
			if i.handlers.OnCreate != nil {
				if err := i.handlers.OnCreate(ctx, e); err != nil {
					handleError(err, e, i.logger)
				}
			}
		case eventsv1.EventType_ContainerSetUpdate:
			if i.handlers.OnUpdate != nil {
				if err := i.handlers.OnUpdate(ctx, e); err != nil {
					handleError(err, e, i.logger)
				}
			}
		case eventsv1.EventType_ContainerSetDelete:
			if i.handlers.OnDelete != nil {
				if err := i.handlers.OnDelete(ctx, e); err != nil {
					handleError(err, e, i.logger)
				}
			}
		}
	}
}

func NewContainerSetEventInformer(h ContainerSetEventHandlerFuncs) EventInformer {
	return &containerSetEventInformer{handlers: h}
}
