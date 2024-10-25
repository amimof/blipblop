package events

import (
	eventsv1 "github.com/amimof/blipblop/api/services/events/v1"
)

var UnimplemetedEventHandler = func(e *eventsv1.Event) {}

type EventInformer interface {
	Run(<-chan *eventsv1.Event)
}

type EventHandlerFunc func(e *eventsv1.Event) error

type ResourceEventHandlerFunc func(e *eventsv1.Event)

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
	OnCreate ResourceEventHandlerFunc
	OnUpdate ResourceEventHandlerFunc
	OnDelete ResourceEventHandlerFunc
	OnJoin   ResourceEventHandlerFunc
	OnForget ResourceEventHandlerFunc
}

type containerEventInformer struct {
	handlers ContainerEventHandlerFuncs
}

func (i *containerEventInformer) Run(eventChan <-chan *eventsv1.Event) {
	for e := range eventChan {
		switch e.Type {
		case eventsv1.EventType_ContainerCreate:
			if i.handlers.OnCreate != nil {
				i.handlers.OnCreate(e)
			}
		case eventsv1.EventType_ContainerUpdate:
			if i.handlers.OnUpdate != nil {
				i.handlers.OnUpdate(e)
			}
		case eventsv1.EventType_ContainerDelete:
			if i.handlers.OnDelete != nil {
				i.handlers.OnDelete(e)
			}
		case eventsv1.EventType_ContainerStart:
			if i.handlers.OnStart != nil {
				i.handlers.OnStart(e)
			}
		case eventsv1.EventType_ContainerKill:
			if i.handlers.OnKill != nil {
				i.handlers.OnKill(e)
			}
		case eventsv1.EventType_ContainerStop:
			if i.handlers.OnStop != nil {
				i.handlers.OnStop(e)
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
				i.handlers.OnCreate(e)
			}
		case eventsv1.EventType_ContainerSetUpdate:
			if i.handlers.OnUpdate != nil {
				i.handlers.OnUpdate(e)
			}
		case eventsv1.EventType_ContainerSetDelete:
			if i.handlers.OnDelete != nil {
				i.handlers.OnDelete(e)
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
				i.handlers.OnCreate(e)
			}
		case eventsv1.EventType_NodeUpdate:
			if i.handlers.OnUpdate != nil {
				i.handlers.OnUpdate(e)
			}
		case eventsv1.EventType_NodeDelete:
			if i.handlers.OnDelete != nil {
				i.handlers.OnDelete(e)
			}
		case eventsv1.EventType_NodeJoin:
			if i.handlers.OnJoin != nil {
				i.handlers.OnJoin(e)
			}
		case eventsv1.EventType_NodeForget:
			if i.handlers.OnForget != nil {
				i.handlers.OnForget(e)
			}
		}
	}
}

func NewContainerEventInformer(h ContainerEventHandlerFuncs) EventInformer {
	return &containerEventInformer{handlers: h}
}

func NewContainerSetEventInformer(h ContainerSetEventHandlerFuncs) EventInformer {
	return &containerSetEventInformer{handlers: h}
}

func NewNodeEventInformer(h NodeEventHandlerFuncs) EventInformer {
	return &nodeEventInformer{handlers: h}
}
