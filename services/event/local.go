package event

import (
	"context"

	eventsv1 "github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/pkg/repository"
	"go.opentelemetry.io/otel"
	"google.golang.org/grpc"
)

type local struct {
	repo repository.EventRepository
	// mu       sync.Mutex
	// exchange *events.Exchange
	// logger   logger.Logger
}

var (
	_      eventsv1.EventServiceClient = &local{}
	tracer                             = otel.GetTracerProvider().Tracer("blipblop-server")
)

func (n *local) Create(ctx context.Context, req *eventsv1.CreateEventRequest, _ ...grpc.CallOption) (*eventsv1.CreateEventResponse, error) {
	ctx, span := tracer.Start(ctx, "event.Create")
	defer span.End()

	err := n.repo.Create(ctx, req.GetEvent())
	if err != nil {
		return nil, err
	}
	return &eventsv1.CreateEventResponse{Event: req.GetEvent()}, nil
}

func (n *local) Get(ctx context.Context, req *eventsv1.GetEventRequest, _ ...grpc.CallOption) (*eventsv1.GetEventResponse, error) {
	ctx, span := tracer.Start(ctx, "event.Get")
	defer span.End()

	e, err := n.repo.Get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &eventsv1.GetEventResponse{Event: e}, nil
}

func (n *local) Delete(ctx context.Context, req *eventsv1.DeleteEventRequest, _ ...grpc.CallOption) (*eventsv1.DeleteEventResponse, error) {
	ctx, span := tracer.Start(ctx, "event.Delete")
	defer span.End()

	err := n.repo.Delete(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &eventsv1.DeleteEventResponse{Id: req.GetId()}, nil
}

func (n *local) List(ctx context.Context, req *eventsv1.ListEventRequest, _ ...grpc.CallOption) (*eventsv1.ListEventResponse, error) {
	ctx, span := tracer.Start(ctx, "event.List")
	defer span.End()

	l, err := n.repo.List(ctx)
	if err != nil {
		return nil, err
	}

	return &eventsv1.ListEventResponse{Events: l}, nil
}

// Publish implements events.EventServiceClient.
func (n *local) Publish(ctx context.Context, req *eventsv1.PublishRequest, _ ...grpc.CallOption) (*eventsv1.PublishResponse, error) {
	panic("unimplemented")
}

// Subscribe implements events.EventServiceClient.
func (n *local) Subscribe(ctx context.Context, in *eventsv1.SubscribeRequest, opts ...grpc.CallOption) (eventsv1.EventService_SubscribeClient, error) {
	panic("unimplemented")
}
