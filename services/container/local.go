package container

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/amimof/blipblop/api/services/containers/v1"
	eventsv1 "github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/pkg/events"
	"github.com/amimof/blipblop/pkg/logger"
	"github.com/amimof/blipblop/pkg/repository"
	"github.com/amimof/blipblop/services"
	"github.com/amimof/blipblop/services/event"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type local struct {
	repo     repository.ContainerRepository
	mu       sync.Mutex
	exchange *events.Exchange
	logger   logger.Logger
}

var _ containers.ContainerServiceClient = &local{}

func (l *local) handleError(err error, msg string, keysAndValues ...any) error {
	def := []any{"error", err.Error()}
	def = append(def, keysAndValues...)
	l.logger.Error(msg, def...)
	if errors.Is(err, repository.ErrNotFound) {
		return status.Error(codes.NotFound, err.Error())
	}
	return status.Error(codes.Internal, err.Error())
}

func (l *local) Get(ctx context.Context, req *containers.GetContainerRequest, _ ...grpc.CallOption) (*containers.GetContainerResponse, error) {
	container, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		return nil, l.handleError(err, "couldn't GET container from repo", "name", req.GetId())
	}
	return &containers.GetContainerResponse{
		Container: container,
	}, nil
}

func (l *local) List(ctx context.Context, req *containers.ListContainerRequest, _ ...grpc.CallOption) (*containers.ListContainerResponse, error) {
	ctrs, err := l.Repo().List(ctx)
	if err != nil {
		return nil, l.handleError(err, "couldn't LIST containers from repo")
	}
	return &containers.ListContainerResponse{
		Containers: ctrs,
	}, nil
}

func (l *local) Create(ctx context.Context, req *containers.CreateContainerRequest, _ ...grpc.CallOption) (*containers.CreateContainerResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	clientId := md.Get("blipblop_client_id")[0]
	container := req.GetContainer()

	if existing, _ := l.Repo().Get(ctx, container.GetMeta().GetName()); existing != nil {
		return nil, fmt.Errorf("container %s already exists", container.GetMeta().GetName())
	}

	err := services.EnsureMetaForContainer(container)
	if err != nil {
		return nil, err
	}

	err = l.Repo().Create(ctx, container)
	if err != nil {
		return nil, l.handleError(err, "couldn't CREATE container in repo", "name", container.GetMeta().GetName())
	}

	err = l.exchange.Publish(ctx, &eventsv1.PublishRequest{Event: event.NewEventFor(clientId, container.GetMeta().GetName(), eventsv1.EventType_ContainerCreate)})
	if err != nil {
		return nil, l.handleError(err, "error publishing CREATE event", "name", container.GetMeta().GetName(), "event", "ContainerCreate")
	}
	return &containers.CreateContainerResponse{
		Container: container,
	}, nil
}

// Delete publishes a delete request and the subscribers are responsible for deleting resources.
// Once they do, they will update there resource with the status Deleted
func (l *local) Delete(ctx context.Context, req *containers.DeleteContainerRequest, _ ...grpc.CallOption) (*containers.DeleteContainerResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	clientId := md.Get("blipblop_client_id")[0]
	container, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		return nil, l.handleError(err, "couldn't GET container from repo", "id", req.GetId())
	}
	err = l.Repo().Delete(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	err = l.exchange.Publish(ctx, &eventsv1.PublishRequest{Event: event.NewEventFor(clientId, req.GetId(), eventsv1.EventType_ContainerDelete)})
	if err != nil {
		return nil, l.handleError(err, "error publishing DELETE event", "name", container.GetMeta().GetName(), "event", "ContainerDelete")
	}
	return &containers.DeleteContainerResponse{
		Id: req.GetId(),
	}, nil
}

func (l *local) Kill(ctx context.Context, req *containers.KillContainerRequest, _ ...grpc.CallOption) (*containers.KillContainerResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	clientId := md.Get("blipblop_client_id")[0]
	evt := eventsv1.EventType_ContainerStop
	if req.GetForceKill() {
		evt = eventsv1.EventType_ContainerKill
	}
	err := l.exchange.Publish(ctx, &eventsv1.PublishRequest{Event: event.NewEventFor(clientId, req.GetId(), evt)})
	if err != nil {
		return nil, l.handleError(err, "error publishing KILL event", "name", req.GetId(), "event", "ContainerKill")
	}
	return &containers.KillContainerResponse{
		Id: req.GetId(),
	}, nil
}

func (l *local) Start(ctx context.Context, req *containers.StartContainerRequest, _ ...grpc.CallOption) (*containers.StartContainerResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	clientId := md.Get("blipblop_client_id")[0]
	err := l.exchange.Publish(ctx, &eventsv1.PublishRequest{Event: event.NewEventFor(clientId, req.GetId(), eventsv1.EventType_ContainerStart)})
	if err != nil {
		return nil, err
	}
	return &containers.StartContainerResponse{
		Id: req.GetId(),
	}, nil
}

func (l *local) Update(ctx context.Context, req *containers.UpdateContainerRequest, _ ...grpc.CallOption) (*containers.UpdateContainerResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	clientId := md.Get("blipblop_client_id")[0]
	updateMask := req.GetUpdateMask()
	updateContainer := req.GetContainer()
	existing, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		return nil, l.handleError(err, "couldn't GET container from repo", "name", updateContainer.GetMeta().GetName())
	}

	if updateMask != nil && updateMask.IsValid(existing) {
		proto.Merge(existing, updateContainer)
	}

	err = l.Repo().Update(ctx, existing)
	if err != nil {
		return nil, l.handleError(err, "couldn't UPDATE container in repo", "name", existing.GetMeta().GetName())
	}
	err = l.exchange.Publish(ctx, &eventsv1.PublishRequest{Event: event.NewEventFor(clientId, req.GetContainer().GetMeta().GetName(), eventsv1.EventType_ContainerUpdate)})
	if err != nil {
		return nil, l.handleError(err, "error publishing UPDATE event", "name", existing.GetMeta().GetName(), "event", "ContainerUpdate")
	}
	return &containers.UpdateContainerResponse{
		Container: existing,
	}, nil
}

func (l *local) Repo() repository.ContainerRepository {
	if l.repo != nil {
		return l.repo
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return repository.NewContainerInMemRepo()
}
