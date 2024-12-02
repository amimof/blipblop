package informer

import (
	"context"

	eventsv1 "github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/pkg/logger"
)

type NewContainerEventInformerOption func(s *containerEventInformer)

func WithContainerEventInformerLogger(l logger.Logger) NewContainerEventInformerOption {
	return func(s *containerEventInformer) {
		s.logger = l
	}
}

type ResourceEventHandlerFunc func(ctx context.Context, e *eventsv1.Event) error

type ContainerEventHandlerFuncs struct {
	OnCreate   ResourceEventHandlerFunc
	OnUpdate   ResourceEventHandlerFunc
	OnDelete   ResourceEventHandlerFunc
	OnStart    ResourceEventHandlerFunc
	OnKill     ResourceEventHandlerFunc
	OnStop     ResourceEventHandlerFunc
	OnSchedule ResourceEventHandlerFunc
}

func handleError(err error, e *eventsv1.Event, i logger.Logger) {
	i.Error("container event informer error calling handler", "type", e.GetType().String(), "error", err)
}

type containerEventInformer struct {
	handlers ContainerEventHandlerFuncs
	logger   logger.Logger
}

func (i *containerEventInformer) Run(ctx context.Context, eventChan <-chan *eventsv1.Event) {
	for e := range eventChan {
		switch e.Type {
		case eventsv1.EventType_Schedule:
			if i.handlers.OnSchedule != nil {
				if err := i.handlers.OnSchedule(ctx, e); err != nil {
					handleError(err, e, i.logger)
				}
			}
		case eventsv1.EventType_ContainerCreate:
			if i.handlers.OnCreate != nil {
				if err := i.handlers.OnCreate(ctx, e); err != nil {
					handleError(err, e, i.logger)
				}
			}
		case eventsv1.EventType_ContainerUpdate:
			if i.handlers.OnUpdate != nil {
				if err := i.handlers.OnUpdate(ctx, e); err != nil {
					handleError(err, e, i.logger)
				}
			}
		case eventsv1.EventType_ContainerDelete:
			if i.handlers.OnDelete != nil {
				if err := i.handlers.OnDelete(ctx, e); err != nil {
					handleError(err, e, i.logger)
				}
			}
		case eventsv1.EventType_ContainerStart:
			if i.handlers.OnStart != nil {
				if err := i.handlers.OnStart(ctx, e); err != nil {
					handleError(err, e, i.logger)
				}
			}
		case eventsv1.EventType_ContainerKill:
			if i.handlers.OnKill != nil {
				if err := i.handlers.OnKill(ctx, e); err != nil {
					handleError(err, e, i.logger)
				}
			}
		case eventsv1.EventType_ContainerStop:
			if i.handlers.OnStop != nil {
				if err := i.handlers.OnStop(ctx, e); err != nil {
					handleError(err, e, i.logger)
				}
			}
		}
	}
}

func NewContainerEventInformer(h ContainerEventHandlerFuncs, opts ...NewContainerEventInformerOption) EventInformer {
	i := &containerEventInformer{handlers: h, logger: logger.ConsoleLogger{}}

	for _, opt := range opts {
		opt(i)
	}

	return i
}
