package v1

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/services/event"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

type EventV1Client struct {
	// name         string
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

func (c *EventV1Client) Subscribe(ctx context.Context, clientId string, receiveChan chan<- *events.Event, errChan chan<- error) error {
	// Create stream
	stream, err := c.eventService.Subscribe(ctx, &events.SubscribeRequest{ClientId: clientId})
	if err != nil {
		return fmt.Errorf("subscribe failed: %v", err)
	}

	// Read from the stream
	for {
		response, err := stream.Recv()

		// Stream closed by server
		if err == io.EOF {
			errChan <- fmt.Errorf("server stream closed")
			break
		}

		// Handle transient errors
		if err != nil {
			if grpcErr, ok := status.FromError(err); ok {
				errChan <- fmt.Errorf("gRPC stream error %v, code %v", grpcErr.Message(), grpcErr.Code())
				if grpcErr.Code() == status.Code(err) {
					errChan <- errors.New("transient error occured, attempting to reconnect")
					time.Sleep(2 * time.Second)
				}
			}
			errChan <- fmt.Errorf("non-gRPC error: %v", err)
			break
		}
		receiveChan <- response
	}

	return nil
}

func NewEventV1Client(conn *grpc.ClientConn) *EventV1Client {
	return &EventV1Client{
		eventService: events.NewEventServiceClient(conn),
	}
}
