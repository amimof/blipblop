package events

import (
	"context"

	eventsv1 "github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/pkg/logger"
)

var UnimplemetedEventHandler = func(e *eventsv1.Event) {}

type EventInformer interface {
	Run(context.Context, <-chan *eventsv1.Event)
}

type EventHandlerFunc func(e *eventsv1.Event) error

type ResourceEventHandlerFunc func(ctx context.Context, e *eventsv1.Event) error

type (
	NewContainerEventInformerOption    func(s *containerEventInformer)
	NewContainerSetEventInformerOption func(s *containerSetEventInformer)
	NewNodeEventInformerOption         func(s *nodeEventInformer)
)

func WithContainerEventInformerLogger(l logger.Logger) NewContainerEventInformerOption {
	return func(s *containerEventInformer) {
		s.logger = l
	}
}

func WithContainerSetEventInformerLogger(l logger.Logger) NewContainerSetEventInformerOption {
	return func(s *containerSetEventInformer) {
		s.logger = l
	}
}

func WithNodeEventInformerLogger(l logger.Logger) NewNodeEventInformerOption {
	return func(s *nodeEventInformer) {
		s.logger = l
	}
}

type ContainerEventHandlerFuncs struct {
	OnCreate ResourceEventHandlerFunc
	OnUpdate ResourceEventHandlerFunc
	OnDelete ResourceEventHandlerFunc
	OnStart  ResourceEventHandlerFunc
	OnKill   ResourceEventHandlerFunc
	OnStop   ResourceEventHandlerFunc
}

type ContainerSetEventHandlerFuncs struct {
	OnCreate ResourceEventHandlerFunc
	OnUpdate ResourceEventHandlerFunc
	OnDelete ResourceEventHandlerFunc
}

type NodeEventHandlerFuncs struct {
	OnCreate  ResourceEventHandlerFunc
	OnUpdate  ResourceEventHandlerFunc
	OnDelete  ResourceEventHandlerFunc
	OnJoin    ResourceEventHandlerFunc
	OnForget  ResourceEventHandlerFunc
	OnConnect ResourceEventHandlerFunc
}

type containerEventInformer struct {
	handlers ContainerEventHandlerFuncs
	logger   logger.Logger
}

func (i *containerEventInformer) Run(ctx context.Context, eventChan <-chan *eventsv1.Event) {
	for e := range eventChan {
		switch e.Type {
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

type nodeEventInformer struct {
	handlers NodeEventHandlerFuncs
	logger   logger.Logger
}

func (i *nodeEventInformer) Run(ctx context.Context, eventChan <-chan *eventsv1.Event) {
	for e := range eventChan {
		switch e.Type {
		case eventsv1.EventType_NodeCreate:
			if i.handlers.OnCreate != nil {
				if err := i.handlers.OnCreate(ctx, e); err != nil {
					handleError(err, e, i.logger)
				}
			}
		case eventsv1.EventType_NodeUpdate:
			if i.handlers.OnUpdate != nil {
				if err := i.handlers.OnUpdate(ctx, e); err != nil {
					handleError(err, e, i.logger)
				}
			}
		case eventsv1.EventType_NodeDelete:
			if i.handlers.OnDelete != nil {
				if err := i.handlers.OnDelete(ctx, e); err != nil {
					handleError(err, e, i.logger)
				}
			}
		case eventsv1.EventType_NodeJoin:
			if i.handlers.OnJoin != nil {
				if err := i.handlers.OnJoin(ctx, e); err != nil {
					handleError(err, e, i.logger)
				}
			}
		case eventsv1.EventType_NodeForget:
			if i.handlers.OnForget != nil {
				if err := i.handlers.OnForget(ctx, e); err != nil {
					handleError(err, e, i.logger)
				}
			}
		case eventsv1.EventType_NodeConnect:
			if i.handlers.OnConnect != nil {
				if err := i.handlers.OnConnect(ctx, e); err != nil {
					handleError(err, e, i.logger)
				}
			}
		}
	}
}

func handleError(err error, e *eventsv1.Event, i logger.Logger) {
	i.Error("container event informer error calling handler", "type", e.GetType().String(), "error", err)
}

func NewContainerEventInformer(h ContainerEventHandlerFuncs, opts ...NewContainerEventInformerOption) EventInformer {
	i := &containerEventInformer{handlers: h, logger: logger.ConsoleLogger{}}

	for _, opt := range opts {
		opt(i)
	}

	return i
}

func NewContainerSetEventInformer(h ContainerSetEventHandlerFuncs) EventInformer {
	return &containerSetEventInformer{handlers: h}
}

func NewNodeEventInformer(h NodeEventHandlerFuncs) EventInformer {
	return &nodeEventInformer{handlers: h}
}
