package container

import (
	"context"
	"errors"
	"fmt"
	"github.com/amimof/blipblop/api/services/containers/v1"
	"github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/services/event"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
	"sync"
	"time"
)

type local struct {
	repo        Repo
	mu          sync.Mutex
	eventClient *event.EventService
}

var _ containers.ContainerServiceClient = &local{}

func (l *local) Get(ctx context.Context, req *containers.GetContainerRequest, _ ...grpc.CallOption) (*containers.GetContainerResponse, error) {
	container, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	if container == nil {
		return nil, errors.New(fmt.Sprintf("container not found %s", req.GetId()))
	}
	_, err = l.eventClient.Publish(ctx, &events.PublishRequest{Event: event.NewEventFor(req.GetId(), events.EventType_ContainerGetAll)})
	if err != nil {
		return nil, err
	}
	return &containers.GetContainerResponse{
		Container: container,
	}, nil
}

func (l *local) List(ctx context.Context, req *containers.ListContainerRequest, _ ...grpc.CallOption) (*containers.ListContainerResponse, error) {
	ctrs, err := l.Repo().GetAll(ctx)
	if err != nil {
		return nil, err
	}
	return &containers.ListContainerResponse{
		Containers: ctrs,
	}, nil
}

func (l *local) Create(ctx context.Context, req *containers.CreateContainerRequest, _ ...grpc.CallOption) (*containers.CreateContainerResponse, error) {
	container := req.GetContainer()
	container.Created = timestamppb.New(time.Now())
	container.Updated = timestamppb.New(time.Now())
	container.Revision = 1
	err := l.Repo().Create(ctx, container)
	if err != nil {
		return nil, err
	}
	container, err = l.Repo().Get(ctx, container.GetName())
	if err != nil {
		return nil, err
	}
	return &containers.CreateContainerResponse{
		Container: container,
	}, nil
}

func (l *local) Delete(ctx context.Context, req *containers.DeleteContainerRequest, _ ...grpc.CallOption) (*containers.DeleteContainerResponse, error) {
	err := l.Repo().Delete(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &containers.DeleteContainerResponse{
		Id: req.GetId(),
	}, nil
}

func (l *local) Kill(ctx context.Context, req *containers.KillContainerRequest, _ ...grpc.CallOption) (*containers.KillContainerResponse, error) {
	return &containers.KillContainerResponse{
		Id: "",
	}, nil
}

func (l *local) Start(ctx context.Context, req *containers.StartContainerRequest, _ ...grpc.CallOption) (*containers.StartContainerResponse, error) {
	return &containers.StartContainerResponse{
		Id: "",
	}, nil
}

func (l *local) Stop(ctx context.Context, req *containers.StopContainerRequest, _ ...grpc.CallOption) (*containers.StopContainerResponse, error) {
	return &containers.StopContainerResponse{
		Id: "",
	}, nil
}

func (l *local) Update(ctx context.Context, req *containers.UpdateContainerRequest, _ ...grpc.CallOption) (*containers.UpdateContainerResponse, error) {
	return &containers.UpdateContainerResponse{
		Container: nil,
	}, nil
}

func (l *local) Repo() Repo {
	if l.repo != nil {
		return l.repo
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return NewInMemRepo()
}
