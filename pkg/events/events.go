package events

import (
	eventsv1 "github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/api/types/v1"
	"github.com/google/uuid"
)

var topic = "test-topic"

type EventHandlerFunc func(e *eventsv1.Event) error

type ResourceEventHandlerFunc func(e *eventsv1.Event)

type ResourceEventHandlerFuncs struct {
	OnCreate ResourceEventHandlerFunc
	OnUpdate ResourceEventHandlerFunc
	OnDelete ResourceEventHandlerFunc
}

type EventBuilder struct {
	objectId string
	// clientId  string
	operation eventsv1.Operation
}

const (
	OperationAdd    eventsv1.Operation = eventsv1.Operation_Create
	OperationUpdate eventsv1.Operation = eventsv1.Operation_Update
	OperationDelete eventsv1.Operation = eventsv1.Operation_Delete
)

func New() *EventBuilder {
	return &EventBuilder{}
}

func NewRequest(op eventsv1.Operation, objectId string) *eventsv1.PublishRequest {
	return &eventsv1.PublishRequest{
		Event: &eventsv1.Event{
			Meta: &types.Meta{
				Name: uuid.New().String(),
			},
			ObjectId:  objectId,
			Operation: op,
		},
	}
}
