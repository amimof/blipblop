package events

import (
	eventsv1 "github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/api/types/v1"
	"github.com/google/uuid"
)

const (
	OperationAdd    eventsv1.Operation = eventsv1.Operation_Create
	OperationUpdate eventsv1.Operation = eventsv1.Operation_Update
	OperationDelete eventsv1.Operation = eventsv1.Operation_Delete
)

func NewRequest(evType eventsv1.EventType, objectId string) *eventsv1.PublishRequest {
	return &eventsv1.PublishRequest{
		Event: &eventsv1.Event{
			Meta: &types.Meta{
				Name: uuid.New().String(),
			},
			ObjectId: objectId,
			Type:     evType,
		},
	}
}
