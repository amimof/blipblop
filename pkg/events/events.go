// Package events provides interfaces and types for working with events
package events

import (
	"context"
	"maps"

	"github.com/google/uuid"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/amimof/voiyd/api/types/v1"
	"github.com/amimof/voiyd/pkg/labels"

	eventsv1 "github.com/amimof/voiyd/api/services/events/v1"
)

const (
	TaskCreate         = eventsv1.EventType_TaskCreate
	TaskDelete         = eventsv1.EventType_TaskDelete
	TaskUpdate         = eventsv1.EventType_TaskUpdate
	TaskStart          = eventsv1.EventType_TaskStart
	TaskGet            = eventsv1.EventType_TaskGet
	TaskList           = eventsv1.EventType_TaskList
	TaskKill           = eventsv1.EventType_TaskKill
	TaskStop           = eventsv1.EventType_TaskStop
	TaskPatch          = eventsv1.EventType_TaskPatch
	NodeGet            = eventsv1.EventType_NodeGet
	NodeCreate         = eventsv1.EventType_NodeCreate
	NodeDelete         = eventsv1.EventType_NodeDelete
	NodeList           = eventsv1.EventType_NodeList
	NodeUpdate         = eventsv1.EventType_NodeUpdate
	NodeJoin           = eventsv1.EventType_NodeJoin
	NodeForget         = eventsv1.EventType_NodeForget
	NodeConnect        = eventsv1.EventType_NodeConnect
	NodeUpgrade        = eventsv1.EventType_NodeUpgrade
	NodePatch          = eventsv1.EventType_NodePatch
	ContainerSetCreate = eventsv1.EventType_ContainerSetCreate
	ContainerSetDelete = eventsv1.EventType_ContainerSetDelete
	ContainerSetUpdate = eventsv1.EventType_ContainerSetUpdate
	Schedule           = eventsv1.EventType_Schedule
	TailLogsStart      = eventsv1.EventType_TailLogsStart
	TailLogsStop       = eventsv1.EventType_TailLogsStop
	VolumeCreate       = eventsv1.EventType_VolumeCreate
	VolumeDelete       = eventsv1.EventType_VolumeDelete
	VolumeUpdate       = eventsv1.EventType_VolumeUpdate
	VolumeGet          = eventsv1.EventType_VolumeGet
	VolumeList         = eventsv1.EventType_VolumeList
)

var ALL = []eventsv1.EventType{
	TaskCreate,
	TaskDelete,
	TaskUpdate,
	TaskStart,
	TaskGet,
	TaskList,
	TaskKill,
	TaskStop,
	TaskPatch,
	NodeGet,
	NodeCreate,
	NodeDelete,
	NodeList,
	NodeUpdate,
	NodeJoin,
	NodeForget,
	NodeConnect,
	NodeUpgrade,
	NodePatch,
	ContainerSetCreate,
	ContainerSetDelete,
	ContainerSetUpdate,
	Schedule,

	TailLogsStart,
	TailLogsStop,

	VolumeCreate,
	VolumeDelete,
	VolumeUpdate,
	VolumeGet,
	VolumeList,
}

type Subscriber interface {
	Subscribe(context.Context, ...eventsv1.EventType) chan *eventsv1.Event
	Unsubscribe(context.Context, eventsv1.EventType) error
}

type Publisher interface {
	Publish(context.Context, *eventsv1.Event) error
}

type Forwarder interface {
	Forward(context.Context, *eventsv1.Event) error
}

type Object protoreflect.ProtoMessage

func NewRequest(evType eventsv1.EventType, obj Object, labels ...map[string]string) *eventsv1.PublishRequest {
	return &eventsv1.PublishRequest{
		Event: NewEvent(evType, obj, labels...),
	}
}

func NewEvent(evType eventsv1.EventType, obj Object, eventLabels ...map[string]string) *eventsv1.Event {
	// Merge the maps
	l := labels.New()
	for _, label := range eventLabels {
		maps.Copy(l, label)
	}
	o, _ := anypb.New(obj)
	return &eventsv1.Event{
		Version: "event/v1",
		Meta: &types.Meta{
			Name:   uuid.New().String(),
			Labels: l,
		},
		Type:   evType,
		Object: o,
	}
}
