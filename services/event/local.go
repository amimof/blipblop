package event

import (
	"context"
	"fmt"
	"sync"

	"github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/pkg/repository"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

type local struct {
	repo repository.Repository
	mu   sync.Mutex
}

var _ events.EventServiceClient = &local{}

func (l *local) Get(ctx context.Context, req *events.GetEventRequest, _ ...grpc.CallOption) (*events.GetEventResponse, error) {
	data, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, fmt.Errorf("event not found %s", req.GetId())
	}

	event := &events.Event{}
	err = proto.Unmarshal(data, event)
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
	data, err := l.Repo().GetAll(ctx)
	if err != nil {
		return nil, err
	}
	res := []*events.Event{}

	for _, b := range data {
		event := &events.Event{}
		err = proto.Unmarshal(b, event)
		if err != nil {
			return nil, err
		}
		res = append(res, event)
	}
	return &events.ListEventResponse{
		Events: res,
	}, nil
}

func (l *local) Subscribe(ctx context.Context, req *events.SubscribeRequest, _ ...grpc.CallOption) (events.EventService_SubscribeClient, error) {
	return nil, nil
}

func (l *local) Publish(ctx context.Context, req *events.PublishRequest, _ ...grpc.CallOption) (*events.PublishResponse, error) {
	event := req.GetEvent()
	data, err := proto.Marshal(event)
	if err != nil {
		return nil, err
	}

	return nil, l.Repo().Create(ctx, event.GetId(), data)
}

func (l *local) Repo() repository.Repository {
	if l.repo != nil {
		return l.repo
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return repository.NewInMemRepo()
}
