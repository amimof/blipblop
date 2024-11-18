package events

import (
	eventsv1 "github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/api/types/v1"
	"github.com/google/uuid"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
)

const (
	OperationAdd    eventsv1.Operation = eventsv1.Operation_Create
	OperationUpdate eventsv1.Operation = eventsv1.Operation_Update
	OperationDelete eventsv1.Operation = eventsv1.Operation_Delete
)

type Object protoreflect.ProtoMessage

func NewRequest(evType eventsv1.EventType, obj Object, labels ...map[string]string) *eventsv1.PublishRequest {
	// Merge the maps
	l := map[string]string{}
	for _, label := range labels {
		for k, v := range label {
			l[k] = v
		}
	}
	o, _ := anypb.New(obj)
	return &eventsv1.PublishRequest{
		Event: &eventsv1.Event{
			Meta: &types.Meta{
				Name:   uuid.New().String(),
				Labels: l,
			},
			Type:   evType,
			Object: o,
		},
	}
}
