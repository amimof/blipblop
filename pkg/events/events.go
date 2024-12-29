package events

import (
	"context"

	eventsv1 "github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/api/types/v1"
	"github.com/google/uuid"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
)

const (
	ContainerCreate    = eventsv1.EventType_ContainerCreate
	ContainerDelete    = eventsv1.EventType_ContainerDelete
	ContainerUpdate    = eventsv1.EventType_ContainerUpdate
	ContainerStart     = eventsv1.EventType_ContainerStart
	ContainerGet       = eventsv1.EventType_ContainerGet
	ContainerList      = eventsv1.EventType_ContainerList
	ContainerKill      = eventsv1.EventType_ContainerKill
	ContainerStop      = eventsv1.EventType_ContainerStop
	NodeGet            = eventsv1.EventType_NodeGet
	NodeCreate         = eventsv1.EventType_NodeCreate
	NodeDelete         = eventsv1.EventType_NodeDelete
	NodeList           = eventsv1.EventType_NodeList
	NodeUpdate         = eventsv1.EventType_NodeUpdate
	NodeJoin           = eventsv1.EventType_NodeJoin
	NodeForget         = eventsv1.EventType_NodeForget
	NodeConnect        = eventsv1.EventType_NodeConnect
	ContainerSetCreate = eventsv1.EventType_ContainerSetCreate
	ContainerSetDelete = eventsv1.EventType_ContainerSetDelete
	ContainerSetUpdate = eventsv1.EventType_ContainerSetUpdate
	Schedule           = eventsv1.EventType_Schedule
)

var ALL = []eventsv1.EventType{
	ContainerCreate,
	ContainerDelete,
	ContainerUpdate,
	ContainerStart,
	ContainerGet,
	ContainerList,
	ContainerKill,
	ContainerStop,
	NodeGet,
	NodeCreate,
	NodeDelete,
	NodeList,
	NodeUpdate,
	NodeJoin,
	NodeForget,
	NodeConnect,
	ContainerSetCreate,
	ContainerSetDelete,
	ContainerSetUpdate,
	Schedule,
}

type Subscriber interface {
	Subscribe(context.Context, ...eventsv1.EventType) chan *eventsv1.Event
	Unsubscribe(context.Context, eventsv1.EventType) error
}

type Publisher interface {
	Publish(context.Context, eventsv1.EventType, *eventsv1.Event) error
}

type Forwarder interface {
	Forward(context.Context, eventsv1.EventType, *eventsv1.Event) error
}

type Object protoreflect.ProtoMessage

func NewRequest(evType eventsv1.EventType, obj Object, labels ...map[string]string) *eventsv1.PublishRequest {
	return &eventsv1.PublishRequest{
		Event: NewEvent(evType, obj, labels...),
	}
}

func NewEvent(evType eventsv1.EventType, obj Object, labels ...map[string]string) *eventsv1.Event {
	// Merge the maps
	l := map[string]string{}
	for _, label := range labels {
		for k, v := range label {
			l[k] = v
		}
	}
	o, _ := anypb.New(obj)
	return &eventsv1.Event{
		Meta: &types.Meta{
			Name:   uuid.New().String(),
			Labels: l,
		},
		Type:   evType,
		Object: o,
	}
}
