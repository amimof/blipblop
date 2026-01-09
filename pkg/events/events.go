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

	RuntimeTaskExit         = eventsv1.EventType_RuntimeTaskExit
	RuntimeTaskCreate       = eventsv1.EventType_RuntimeTaskCreate
	RuntimeTaskStart        = eventsv1.EventType_RuntimeTaskStart
	RuntimeTaskDelete       = eventsv1.EventType_RuntimeTaskDelete
	RuntimeTaskIO           = eventsv1.EventType_RuntimeTaskIO
	RuntimeTaskOOM          = eventsv1.EventType_RuntimeTaskOOM
	RuntimeTaskExecAdded    = eventsv1.EventType_RuntimeTaskExecAdded
	RuntimeTaskExecStarted  = eventsv1.EventType_RuntimeTaskExecStarted
	RuntimeTaskPaused       = eventsv1.EventType_RuntimeTaskPaused
	RuntimeTaskResumed      = eventsv1.EventType_RuntimeTaskResumed
	RuntimeTaskCheckpointed = eventsv1.EventType_RuntimeTaskCheckpointed
	RuntimeSnapshotPrepare  = eventsv1.EventType_RuntimeSnapshotPrepare
	RuntimeSnapshotCommit   = eventsv1.EventType_RuntimeSnapshotCommit
	RuntimeSnapshotRemove   = eventsv1.EventType_RuntimeSnapshotRemove
	RuntimeNamespaceCreate  = eventsv1.EventType_RuntimeNamespaceCreate
	RuntimeNamespaceUpdate  = eventsv1.EventType_RuntimeNamespaceUpdate
	RuntimeNamespaceDelete  = eventsv1.EventType_RuntimeNamespaceDelete
	RuntimeImageCreate      = eventsv1.EventType_RuntimeImageCreate
	RuntimeImageUpdate      = eventsv1.EventType_RuntimeImageUpdate
	RuntimeImageDelete      = eventsv1.EventType_RuntimeImageDelete
	RuntimeContainerCreate  = eventsv1.EventType_RuntimeContainerCreate
	RuntimeContainerUpdate  = eventsv1.EventType_RuntimeContainerUpdate
	RuntimeContainerDelete  = eventsv1.EventType_RuntimeContainerDelete
	RuntimeContentCreate    = eventsv1.EventType_RuntimeContentCreate
	RuntimeContentDelete    = eventsv1.EventType_RuntimeContentDelete
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

	RuntimeTaskExit,
	RuntimeTaskCreate,
	RuntimeTaskStart,
	RuntimeTaskDelete,
	RuntimeTaskIO,
	RuntimeTaskOOM,
	RuntimeTaskExecAdded,
	RuntimeTaskExecStarted,
	RuntimeTaskPaused,
	RuntimeTaskResumed,
	RuntimeTaskCheckpointed,
	RuntimeSnapshotPrepare,
	RuntimeSnapshotCommit,
	RuntimeSnapshotRemove,
	RuntimeNamespaceCreate,
	RuntimeNamespaceUpdate,
	RuntimeNamespaceDelete,
	RuntimeImageCreate,
	RuntimeImageUpdate,
	RuntimeImageDelete,
	RuntimeContainerCreate,
	RuntimeContainerUpdate,
	RuntimeContainerDelete,
	RuntimeContentCreate,
	RuntimeContentDelete,
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
