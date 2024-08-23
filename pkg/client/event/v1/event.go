package v1

import (
	"context"
	"io"
	"log"

	"github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/services/event"
	"google.golang.org/grpc"
)

type EventV1Client struct {
	name         string
	eventService events.EventServiceClient
}

func (c *EventV1Client) EventService() events.EventServiceClient {
	return c.eventService
}

func (c *EventV1Client) Publish(ctx context.Context, id string, evt events.EventType) error {
	req := &events.PublishRequest{Event: event.NewEventFor(id, evt)}
	_, err := c.eventService.Publish(ctx, req)
	if err != nil {
		return err
	}
	return nil
}

func (c *EventV1Client) Subscribe(ctx context.Context) (<-chan *events.Event, <-chan error) {
	evc := make(chan *events.Event)
	errc := make(chan error)
	stream, err := c.eventService.Subscribe(ctx, &events.SubscribeRequest{Id: c.name})
	if err != nil {
		log.Fatalf("subscribe error occurred %s", err.Error())
	}
	go func() {
		for {
			in, err := stream.Recv()
			if err == io.EOF {
				errc <- err
			}
			if err != nil {
				errc <- err
			}
			evc <- in
		}
	}()
	return evc, errc
}

func NewEventV1Client(conn *grpc.ClientConn) *EventV1Client {
	return &EventV1Client{
		eventService: events.NewEventServiceClient(conn),
	}
}
