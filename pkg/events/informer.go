package events

import (
	eventsv1 "github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/pkg/logger"
)

var UnimplemetedEventHandler = func(e *eventsv1.Event) {}

type EventInformer interface {
	Run(<-chan *eventsv1.Event)
}

type EventHandlerFunc func(e *eventsv1.Event) error

type ResourceEventHandlerFunc func(e *eventsv1.Event) error

type NewContainerEventInformerOption func(s *containerEventInformer)

func WithContainerEventInformerLogger(l logger.Logger) NewContainerEventInformerOption {
	return func(s *containerEventInformer) {
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

func (i *containerEventInformer) Run(eventChan <-chan *eventsv1.Event) {
	for e := range eventChan {
		switch e.Type {
		case eventsv1.EventType_ContainerCreate:
			if i.handlers.OnCreate != nil {
				if err := i.handlers.OnCreate(e); err != nil {
					i.logger.Error("container event informer error calling handler", "type", e.GetType().String(), "error", err)
				}
			}
		case eventsv1.EventType_ContainerUpdate:
			if i.handlers.OnUpdate != nil {
				if err := i.handlers.OnUpdate(e); err != nil {
					i.logger.Error("container event informer error calling handler", "type", e.GetType().String(), "error", err)
				}
			}
		case eventsv1.EventType_ContainerDelete:
			if i.handlers.OnDelete != nil {
				if err := i.handlers.OnDelete(e); err != nil {
					i.logger.Error("container event informer error calling handler", "type", e.GetType().String(), "error", err)
				}
			}
		case eventsv1.EventType_ContainerStart:
			if i.handlers.OnStart != nil {
				if err := i.handlers.OnStart(e); err != nil {
					i.logger.Error("container event informer error calling handler", "type", e.GetType().String(), "error", err)
				}
			}
		case eventsv1.EventType_ContainerKill:
			if i.handlers.OnKill != nil {
				if err := i.handlers.OnKill(e); err != nil {
					i.logger.Error("container event informer error calling handler", "type", e.GetType().String(), "error", err)
				}
			}
		case eventsv1.EventType_ContainerStop:
			if i.handlers.OnStop != nil {
				if err := i.handlers.OnStop(e); err != nil {
					i.logger.Error("container event informer error calling handler", "type", e.GetType().String(), "error", err)
				}
			}
		}
	}
}

type containerSetEventInformer struct {
	handlers ContainerSetEventHandlerFuncs
}

func (i *containerSetEventInformer) Run(eventChan <-chan *eventsv1.Event) {
	for e := range eventChan {
		switch e.Type {
		case eventsv1.EventType_ContainerSetCreate:
			if i.handlers.OnCreate != nil {
				_ = i.handlers.OnCreate(e)
			}
		case eventsv1.EventType_ContainerSetUpdate:
			if i.handlers.OnUpdate != nil {
				_ = i.handlers.OnUpdate(e)
			}
		case eventsv1.EventType_ContainerSetDelete:
			if i.handlers.OnDelete != nil {
				_ = i.handlers.OnDelete(e)
			}
		}
	}
}

type nodeEventInformer struct {
	handlers NodeEventHandlerFuncs
}

func (i *nodeEventInformer) Run(eventChan <-chan *eventsv1.Event) {
	for e := range eventChan {
		switch e.Type {
		case eventsv1.EventType_NodeCreate:
			if i.handlers.OnCreate != nil {
				_ = i.handlers.OnCreate(e)
			}
		case eventsv1.EventType_NodeUpdate:
			if i.handlers.OnUpdate != nil {
				_ = i.handlers.OnUpdate(e)
			}
		case eventsv1.EventType_NodeDelete:
			if i.handlers.OnDelete != nil {
				_ = i.handlers.OnDelete(e)
			}
		case eventsv1.EventType_NodeJoin:
			if i.handlers.OnJoin != nil {
				_ = i.handlers.OnJoin(e)
			}
		case eventsv1.EventType_NodeForget:
			if i.handlers.OnForget != nil {
				_ = i.handlers.OnForget(e)
			}
		case eventsv1.EventType_NodeConnect:
			if i.handlers.OnConnect != nil {
				_ = i.handlers.OnConnect(e)
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

func NewContainerSetEventInformer(h ContainerSetEventHandlerFuncs) EventInformer {
	return &containerSetEventInformer{handlers: h}
}

func NewNodeEventInformer(h NodeEventHandlerFuncs) EventInformer {
	return &nodeEventInformer{handlers: h}
}
