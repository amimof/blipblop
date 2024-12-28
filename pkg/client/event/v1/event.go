package v1

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	eventsv1 "github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/pkg/events"
	"github.com/amimof/blipblop/pkg/labels"
	"github.com/amimof/blipblop/pkg/util"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type ClientV1 struct {
	id           string
	eventService eventsv1.EventServiceClient
	stream       eventsv1.EventService_SubscribeClient
	exchange     events.Exchange
}

func (c *ClientV1) EventService() eventsv1.EventServiceClient {
	return c.eventService
}

func (c *ClientV1) Get(ctx context.Context, id string) (*eventsv1.Event, error) {
	resp, err := c.eventService.Get(ctx, &eventsv1.GetEventRequest{Id: id})
	if err != nil {
		return nil, err
	}
	return resp.GetEvent(), nil
}

func (c *ClientV1) List(ctx context.Context, filter ...labels.Label) ([]*eventsv1.Event, error) {
	l := util.MergeLabels(filter...)
	resp, err := c.eventService.List(ctx, &eventsv1.ListEventRequest{Selector: l})
	if err != nil {
		return nil, err
	}
	return resp.GetEvents(), nil
}

func (c *ClientV1) Create(ctx context.Context, e *eventsv1.Event) (*eventsv1.Event, error) {
	resp, err := c.eventService.Create(ctx, &eventsv1.CreateEventRequest{Event: e})
	if err != nil {
		return nil, err
	}

	return resp.GetEvent(), nil
}

func (c *ClientV1) Publish(ctx context.Context, obj events.Object, evt eventsv1.EventType) error {
	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_client_id", c.id)
	req := events.NewRequest(evt, obj)
	// re := &eventsv1.PublishRequest{Event: event.NewEventFor(c.id, id, evt)}
	_, err := c.eventService.Publish(ctx, req)
	if err != nil {
		return err
	}
	return nil
}

// On registers a handler func for a certain event type
func (c *ClientV1) On(ev eventsv1.EventType, f events.HandlerFunc) {
	c.exchange.On(ev, f)
}

func (c *ClientV1) Once(ev eventsv1.EventType, f events.HandlerFunc) {
	c.exchange.On(ev, f)
}

func (c *ClientV1) Subscribe(ctx context.Context, topics ...eventsv1.EventType) (chan *eventsv1.Event, chan error) {
	errChan := make(chan error, 10)
	bus := c.exchange.Subscribe(ctx, topics...)

	go func() {
		for {
			// Check if the context is already canceled before starting a connection
			select {
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			default:
			}

			// Start a new stream connection
			stream, err := c.startStream(ctx)
			if err != nil {
				time.Sleep(2 * time.Second)
				continue
			}

			// Stream handling
			streamErr := c.handleStream(ctx, stream, errChan)

			// Log and retry on transiet errors
			if streamErr != nil {

				// Stream closed due to context cancellation
				if errors.Is(streamErr, context.Canceled) {
					errChan <- err
					return
				}

				// Backoff reconnect
				time.Sleep(2 * time.Second)
			}

		}
	}()
	return bus, errChan
}

func (c *ClientV1) startStream(ctx context.Context) (eventsv1.EventService_SubscribeClient, error) {
	stream, err := c.eventService.Subscribe(ctx, &eventsv1.SubscribeRequest{ClientId: c.id})
	if err != nil {
		return nil, fmt.Errorf("failed to create stream: %v", err)
	}
	return stream, nil
}

func (c *ClientV1) handleStream(ctx context.Context, stream eventsv1.EventService_SubscribeClient, errChan chan<- error) error {
	// Start receiving messages from the server
	for {
		select {
		case <-ctx.Done():
			return context.Canceled
		default:
			response, err := stream.Recv()
			if err != nil {

				// Handle EOF and retryable gRPC errors
				if errors.Is(err, io.EOF) {
					return io.EOF
				}

				// Transient stream error
				if s, ok := status.FromError(err); ok && isRetryableError(s.Code()) {
					return fmt.Errorf("transient stream error %s %s: %v", s.Message(), s.Code(), err)
				}

				// Non-retryable error
				errChan <- err
				return err
			}
			// Send received message to chan
			if err := c.exchange.Publish(ctx, response.GetType(), response); err != nil {
				errChan <- err
			}

		}
	}
}

func isRetryableError(code codes.Code) bool {
	return code == codes.Unavailable || code == codes.ResourceExhausted || code == codes.Internal
}

func NewClientV1(conn *grpc.ClientConn, clientId string) *ClientV1 {
	if clientId == "" {
		clientId = uuid.New().String()
	}
	return &ClientV1{
		eventService: eventsv1.NewEventServiceClient(conn),
		id:           clientId,
		exchange:     *events.NewExchange(),
	}
}
