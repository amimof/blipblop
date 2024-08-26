package container

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/amimof/blipblop/api/services/containers/v1"
	"github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/services/event"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
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
		return nil, fmt.Errorf("container not found %s", req.GetId())
	}
	_, err = l.eventClient.Publish(ctx, &events.PublishRequest{Event: event.NewEventFor(req.GetId(), events.EventType_ContainerGet)})
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
	_, err = l.eventClient.Publish(ctx, &events.PublishRequest{Event: event.NewEventFor("", events.EventType_ContainerList)})
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
	_, err = l.eventClient.Publish(ctx, &events.PublishRequest{Event: event.NewEventFor(container.GetName(), events.EventType_ContainerCreate)})
	if err != nil {
		return nil, err
	}
	return &containers.CreateContainerResponse{
		Container: container,
	}, nil
}

// Delete publishes a delete request and the subscribers are responsible for deleting resources.
// Once they do, they will update there resource with the status Deleted
func (l *local) Delete(ctx context.Context, req *containers.DeleteContainerRequest, _ ...grpc.CallOption) (*containers.DeleteContainerResponse, error) {
	_, err := l.eventClient.Publish(ctx, &events.PublishRequest{Event: event.NewEventFor(req.GetId(), events.EventType_ContainerDelete)})
	if err != nil {
		return nil, err
	}
	err = l.Repo().Delete(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &containers.DeleteContainerResponse{
		Id: req.GetId(),
	}, nil
}

func (l *local) Kill(ctx context.Context, req *containers.KillContainerRequest, _ ...grpc.CallOption) (*containers.KillContainerResponse, error) {
	_, err := l.eventClient.Publish(ctx, &events.PublishRequest{Event: event.NewEventFor(req.GetId(), events.EventType_ContainerKill)})
	if err != nil {
		return nil, err
	}
	return &containers.KillContainerResponse{
		Id: req.GetId(),
	}, nil
}

func (l *local) Start(ctx context.Context, req *containers.StartContainerRequest, _ ...grpc.CallOption) (*containers.StartContainerResponse, error) {
	_, err := l.eventClient.Publish(ctx, &events.PublishRequest{Event: event.NewEventFor(req.GetId(), events.EventType_ContainerStart)})
	if err != nil {
		return nil, err
	}
	return &containers.StartContainerResponse{
		Id: req.GetId(),
	}, nil
}

func (l *local) Update(ctx context.Context, req *containers.UpdateContainerRequest, _ ...grpc.CallOption) (*containers.UpdateContainerResponse, error) {
	updateMask := req.GetUpdateMask()
	updateContainer := req.GetContainer()
	existing, err := l.Repo().Get(ctx, updateContainer.GetName())
	if err != nil {
		return nil, err
	}
	if updateMask != nil && updateMask.IsValid(existing) {
		proto.Merge(existing, updateContainer)
	}
	err = l.Repo().Update(ctx, existing)
	if err != nil {
		return nil, err
	}
	_, err = l.eventClient.Publish(ctx, &events.PublishRequest{Event: event.NewEventFor(req.GetContainer().GetName(), events.EventType_ContainerUpdate)})
	if err != nil {
		return nil, err
	}
	return &containers.UpdateContainerResponse{
		Container: existing,
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
