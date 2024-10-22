package containerset

import (
	"context"
	"errors"
	"fmt"
	"sync"

	containersetsv1 "github.com/amimof/blipblop/api/services/containersets/v1"
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
	repo     repository.ContainerSetRepository
	mu       sync.Mutex
	exchange *events.Exchange
	logger   logger.Logger
}

var _ containersetsv1.ContainerSetServiceClient = &local{}

func (l *local) handleError(err error, msg string, keysAndValues ...any) error {
	def := []any{"error", err.Error()}
	def = append(def, keysAndValues...)
	l.logger.Error(msg, def...)
	if errors.Is(err, repository.ErrNotFound) {
		return status.Error(codes.NotFound, err.Error())
	}
	return status.Error(codes.Internal, err.Error())
}

func (l *local) Get(ctx context.Context, req *containersetsv1.GetContainerSetRequest, _ ...grpc.CallOption) (*containersetsv1.GetContainerSetResponse, error) {
	container, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		return nil, l.handleError(err, "couldn't GET container from repo", "name", req.GetId())
	}
	return &containersetsv1.GetContainerSetResponse{
		ContainerSet: container,
	}, nil
}

func (l *local) List(ctx context.Context, req *containersetsv1.ListContainerSetRequest, _ ...grpc.CallOption) (*containersetsv1.ListContainerSetResponse, error) {
	ctrs, err := l.Repo().List(ctx)
	if err != nil {
		return nil, l.handleError(err, "couldn't LIST containers from repo")
	}
	return &containersetsv1.ListContainerSetResponse{
		ContainerSets: ctrs,
	}, nil
}

func (l *local) Create(ctx context.Context, req *containersetsv1.CreateContainerSetRequest, _ ...grpc.CallOption) (*containersetsv1.CreateContainerSetResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	clientId := md.Get("blipblop_client_id")[0]
	container := req.GetContainerSet()

	if existing, _ := l.Repo().Get(ctx, container.GetMeta().GetName()); existing != nil {
		return nil, fmt.Errorf("container %s already exists", container.GetMeta().GetName())
	}

	m, err := services.EnsureMeta(container)
	if err != nil {
		return nil, err
	}
	container.Meta = m

	err = l.Repo().Create(ctx, container)
	if err != nil {
		return nil, l.handleError(err, "couldn't CREATE container in repo", "name", container.GetMeta().GetName())
	}

	err = l.exchange.Publish(ctx, &eventsv1.PublishRequest{Event: event.NewEventFor(clientId, container.GetMeta().GetName(), eventsv1.EventType_ContainerCreate)})
	if err != nil {
		return nil, l.handleError(err, "error publishing CREATE event", "name", container.GetMeta().GetName(), "event", "ContainerCreate")
	}
	return &containersetsv1.CreateContainerSetResponse{
		ContainerSet: container,
	}, nil
}

// Delete publishes a delete request and the subscribers are responsible for deleting resources.
// Once they do, they will update there resource with the status Deleted
func (l *local) Delete(ctx context.Context, req *containersetsv1.DeleteContainerSetRequest, _ ...grpc.CallOption) (*containersetsv1.DeleteContainerSetResponse, error) {
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
	return &containersetsv1.DeleteContainerSetResponse{
		Id: req.GetId(),
	}, nil
}

func (l *local) Update(ctx context.Context, req *containersetsv1.UpdateContainerSetRequest, _ ...grpc.CallOption) (*containersetsv1.UpdateContainerSetResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	clientId := md.Get("blipblop_client_id")[0]
	updateMask := req.GetUpdateMask()
	updateContainer := req.GetContainerSet()
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
	err = l.exchange.Publish(ctx, &eventsv1.PublishRequest{Event: event.NewEventFor(clientId, req.GetContainerSet().GetMeta().GetName(), eventsv1.EventType_ContainerUpdate)})
	if err != nil {
		return nil, l.handleError(err, "error publishing UPDATE event", "name", existing.GetMeta().GetName(), "event", "ContainerUpdate")
	}
	return &containersetsv1.UpdateContainerSetResponse{
		ContainerSet: existing,
	}, nil
}

func (l *local) Repo() repository.ContainerSetRepository {
	if l.repo != nil {
		return l.repo
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return repository.NewContainerSetInMemRepo()
}
