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

type ClientV1 struct {
	id           string
	eventService events.EventServiceClient
}

func (c *ClientV1) EventService() events.EventServiceClient {
	return c.eventService
}

func (c *ClientV1) Publish(ctx context.Context, id string, evt events.EventType) error {
	req := &events.PublishRequest{Event: event.NewEventFor(c.id, id, evt)}
	_, err := c.eventService.Publish(ctx, req)
	if err != nil {
		return err
	}
	return nil
}

func (c *ClientV1) Subscribe2(ctx context.Context, receiveChan chan<- *events.Event, errChan chan<- error) error {
	// Create stream
	stream, err := c.eventService.Subscribe(ctx, &events.SubscribeRequest{ClientId: c.id})
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
		if c.id != response.ClientId {
			receiveChan <- response
		}
	}

	return nil
}

func (c *ClientV1) Subscribe(ctx context.Context, receiveChan chan<- *events.Event, errChan chan<- error) error {
	// Create stream
	stream, err := c.eventService.Subscribe(ctx, &events.SubscribeRequest{ClientId: c.id})
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
		if c.id != response.ClientId {
			receiveChan <- response
		}
	}

	return nil
}

func (c *ClientV1) Wait(t events.EventType, id string) error {
	evt := make(chan *events.Event)
	errChan := make(chan error)

	go func() {
		err := c.Subscribe(context.Background(), evt, errChan)
		if err != nil {
			errChan <- fmt.Errorf("error subscribing to events as %s: %v", c.id, err)
			return
		}
	}()

	for {
		select {
		case e := <-evt:
			if e.Type == t && e.GetObjectId() == id {
				return nil
			}
		case err := <-errChan:
			return fmt.Errorf("error waiting for condition %s: %v", t, err)
		case <-time.After(30 * time.Second):
			return errors.New("timout waiting for event")
		}
	}
}

func NewClientV1(conn *grpc.ClientConn, clientId string) *ClientV1 {
	return &ClientV1{
		eventService: events.NewEventServiceClient(conn),
		id:           clientId,
	}
}
