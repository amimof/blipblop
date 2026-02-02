package event

import (
	"context"

	eventsv1 "github.com/amimof/voiyd/api/services/events/v1"
	"github.com/amimof/voiyd/pkg/keys"
	"github.com/amimof/voiyd/pkg/repository"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type local struct {
	repo *repository.Repo[*eventsv1.Event]
}

var (
	_      eventsv1.EventServiceClient = &local{}
	tracer                             = otel.Tracer("service.event")
)

func (n *local) Create(ctx context.Context, req *eventsv1.CreateRequest, _ ...grpc.CallOption) (*eventsv1.CreateResponse, error) {
	ctx, span := tracer.Start(ctx, "event.Create")
	defer span.End()

	ev := req.GetEvent()
	uid := uuid.New().String()

	if ev.GetMeta().GetName() == "" {
		ev.GetMeta().Name = uid
	}

	ev.GetMeta().Created = timestamppb.Now()
	ev.GetMeta().Updated = timestamppb.Now()
	ev.GetMeta().ResourceVersion = 1
	ev.GetMeta().Generation = 1
	ev.GetMeta().Uid = uid

	newEvent, err := n.repo.Create(ctx, req.GetEvent())
	if err != nil {
		return nil, err
	}
	return &eventsv1.CreateResponse{Event: newEvent}, nil
}

func (n *local) Get(ctx context.Context, req *eventsv1.GetRequest, _ ...grpc.CallOption) (*eventsv1.GetResponse, error) {
	ctx, span := tracer.Start(ctx, "event.Get")
	defer span.End()

	uid, err := keys.FromUIDOrName(req.GetUid(), "")
	if err != nil {
		return nil, err
	}

	e, err := n.repo.Get(ctx, uid)
	if err != nil {
		return nil, err
	}
	return &eventsv1.GetResponse{Event: e}, nil
}

func (n *local) Delete(ctx context.Context, req *eventsv1.DeleteRequest, _ ...grpc.CallOption) (*eventsv1.DeleteResponse, error) {
	ctx, span := tracer.Start(ctx, "event.Delete")
	defer span.End()

	uid, err := keys.FromUIDOrName(req.GetUid(), "")
	if err != nil {
		return nil, err
	}

	err = n.repo.Delete(ctx, uid)
	if err != nil {
		return nil, err
	}
	return &eventsv1.DeleteResponse{Uid: req.GetUid()}, nil
}

func (n *local) List(ctx context.Context, req *eventsv1.ListRequest, _ ...grpc.CallOption) (*eventsv1.ListResponse, error) {
	ctx, span := tracer.Start(ctx, "event.List")
	defer span.End()

	// Default to 100 max results
	if req.GetLimit() == 0 {
		req.Limit = 100
	}

	l, err := n.repo.List(ctx, int(req.GetLimit()))
	if err != nil {
		return nil, err
	}

	return &eventsv1.ListResponse{Events: l}, nil
}

// Publish implements events.EventServiceClient.
func (n *local) Publish(ctx context.Context, req *eventsv1.PublishRequest, _ ...grpc.CallOption) (*eventsv1.PublishResponse, error) {
	panic("unimplemented")
}

// Subscribe implements events.EventServiceClient.
func (n *local) Subscribe(ctx context.Context, in *eventsv1.SubscribeRequest, opts ...grpc.CallOption) (eventsv1.EventService_SubscribeClient, error) {
	panic("unimplemented")
}
