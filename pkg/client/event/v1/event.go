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

func (c *EventV1Client) Subscribe(ctx context.Context, receiveChan chan<- *events.Event, errChan chan<- error) error {
	// Create stream
	stream, err := c.eventService.Subscribe(ctx, &events.SubscribeRequest{Id: c.name})
	if err != nil {
		return fmt.Errorf("subscribe failed: %v", err)
	}

	// Read from the stream
	for {
		response, err := stream.Recv()

		// Stream closed by server
		if err == io.EOF {
			errChan <- fmt.Errorf("server stream closed")
			// return fmt.Errorf("stream closed by server")
			break
		}

		// Handle transient errors
		if err != nil {
			if grpcErr, ok := status.FromError(err); ok {
				// log.Printf("gRPC stream error %v, code %v", grpcErr.Message(), grpcErr.Code())
				errChan <- fmt.Errorf("gRPC stream error %v, code %v", grpcErr.Message(), grpcErr.Code())
				if grpcErr.Code() == status.Code(err) {
					// log.Printf("transiet error occured, attempting to reconnect")
					errChan <- errors.New("transient error occured, attempting to reconnect")
					time.Sleep(2 * time.Second)
				}
			}
			// errc <- fmt.Errorf("non-gRPC error: %v", err)
			// log.Printf("non-gRPC error: %v", err)
			errChan <- fmt.Errorf("non-gRPC error: %v", err)
			break
		}
		receiveChan <- response
	}
	// return nil
	// }()
	// return evc, errc
	return nil
}

func NewEventV1Client(conn *grpc.ClientConn) *EventV1Client {
	return &EventV1Client{
		eventService: events.NewEventServiceClient(conn),
	}
}
