package container

import (
	"context"
	"fmt"
	"sync"

	"github.com/amimof/blipblop/api/services/containers/v1"
	"github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/pkg/logger"
	"github.com/amimof/blipblop/pkg/repository"
	"github.com/amimof/blipblop/services"
	"github.com/amimof/blipblop/services/event"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

type local struct {
	repo        repository.ContainerRepository
	mu          sync.Mutex
	eventClient *event.EventService
	logger      logger.Logger
}

var _ containers.ContainerServiceClient = &local{}

func (l *local) handleError(err error, msg string, keysAndValues ...any) error {
	def := []any{"error", err.Error()}
	def = append(def, keysAndValues...)
	l.logger.Error(msg, def...)
	return err
}

func (l *local) Get(ctx context.Context, req *containers.GetContainerRequest, _ ...grpc.CallOption) (*containers.GetContainerResponse, error) {
	container, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		return nil, l.handleError(err, "couldn't GET container from repo", "name", req.GetId())
	}
	if container == nil {
		return nil, fmt.Errorf("container not found %s", req.GetId())
	}
	_, err = l.eventClient.Publish(ctx, &events.PublishRequest{Event: event.NewEventFor(req.GetId(), events.EventType_ContainerGet)})
	if err != nil {
		return nil, l.handleError(err, "error publishing GET event", "id", req.GetId(), "event", "ContainerGet")
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
	_, err = l.eventClient.Publish(ctx, &events.PublishRequest{Event: event.NewEventFor("", events.EventType_ContainerList)})
	if err != nil {
		return nil, l.handleError(err, "error publishing LIST event", "event", "ContainerList")
	}
	return &containers.ListContainerResponse{
		Containers: ctrs,
	}, nil
}

func (l *local) Create(ctx context.Context, req *containers.CreateContainerRequest, _ ...grpc.CallOption) (*containers.CreateContainerResponse, error) {
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

	_, err = l.eventClient.Publish(ctx, &events.PublishRequest{Event: event.NewEventFor(container.GetMeta().GetName(), events.EventType_ContainerCreate)})
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
	container, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		return nil, l.handleError(err, "couldn't GET container from repo", "id", req.GetId())
	}
	if container.GetStatus().GetPhase() == "running" {
		return nil, fmt.Errorf("unable to delete running container %s", req.GetId())
	}
	err = l.Repo().Delete(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	_, err = l.eventClient.Publish(ctx, &events.PublishRequest{Event: event.NewEventFor(req.GetId(), events.EventType_ContainerDelete)})
	if err != nil {
		return nil, l.handleError(err, "error publishing DELETE event", "name", container.GetMeta().GetName(), "event", "ContainerDelete")
	}
	return &containers.DeleteContainerResponse{
		Id: req.GetId(),
	}, nil
}

func (l *local) Kill(ctx context.Context, req *containers.KillContainerRequest, _ ...grpc.CallOption) (*containers.KillContainerResponse, error) {
	evt := events.EventType_ContainerStop
	if req.GetForceKill() {
		evt = events.EventType_ContainerKill
	}
	_, err := l.eventClient.Publish(ctx, &events.PublishRequest{Event: event.NewEventFor(req.GetId(), evt)})
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
	_, err = l.eventClient.Publish(ctx, &events.PublishRequest{Event: event.NewEventFor(req.GetContainer().GetMeta().GetName(), events.EventType_ContainerUpdate)})
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
