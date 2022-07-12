package event

import (
	"context"
	"github.com/amimof/blipblop/api/services/events/v1"
	"google.golang.org/grpc"
	"sync"
)

type local struct {
	repo Repo
	mu   sync.Mutex
}

var _ events.EventServiceClient = &local{}

func (l *local) Get(ctx context.Context, req *events.GetEventRequest, _ ...grpc.CallOption) (*events.GetEventResponse, error) {
	event, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &events.GetEventResponse{
		Event: event,
	}, nil
}

func (l *local) Delete(ctx context.Context, req *events.DeleteEventRequest, _ ...grpc.CallOption) (*events.DeleteEventResponse, error) {
	err := l.Repo().Delete(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &events.DeleteEventResponse{
		Id: req.GetId(),
	}, nil
}

func (l *local) List(ctx context.Context, req *events.ListEventRequest, _ ...grpc.CallOption) (*events.ListEventResponse, error) {
	res, err := l.Repo().List(ctx)
	if err != nil {
		return nil, err
	}
	return &events.ListEventResponse{
		Events: res,
	}, nil
}

func (l *local) Subscribe(ctx context.Context, req *events.SubscribeRequest, _ ...grpc.CallOption) (events.EventService_SubscribeClient, error) {
	return nil, nil
}

func (l *local) Publish(ctx context.Context, req *events.PublishRequest, _ ...grpc.CallOption) (*events.PublishResponse, error) {
	l.Repo().Create(ctx, req.GetEvent())
	return nil, nil
}

func (l *local) Repo() Repo {
	if l.repo != nil {
		return l.repo
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return NewInMemRepo()
}
