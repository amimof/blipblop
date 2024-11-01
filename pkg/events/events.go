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

func NewRequest(evType eventsv1.EventType, obj Object) *eventsv1.PublishRequest {
	o, _ := anypb.New(obj)
	return &eventsv1.PublishRequest{
		Event: &eventsv1.Event{
			Meta: &types.Meta{
				Name: uuid.New().String(),
			},
			Type:   evType,
			Object: o,
		},
	}
}
