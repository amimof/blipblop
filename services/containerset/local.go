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
	"github.com/amimof/blipblop/pkg/protoutils"
	"github.com/amimof/blipblop/pkg/repository"
	"github.com/amimof/blipblop/services"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
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
	containerSet := req.GetContainerSet()
	containerSetId := containerSet.GetMeta().GetName()

	if existing, _ := l.Repo().Get(ctx, containerSet.GetMeta().GetName()); existing != nil {
		return nil, fmt.Errorf("container set %s already exists", containerSet.GetMeta().GetName())
	}
	var err error
	containerSet.Meta, err = services.EnsureMeta(containerSet)
	if err != nil {
		return nil, err
	}

	err = l.Repo().Create(ctx, containerSet)
	if err != nil {
		return nil, l.handleError(err, "couldn't CREATE container in repo", "name", containerSet.GetMeta().GetName())
	}

	err = l.exchange.Publish(ctx, events.NewRequest(eventsv1.EventType_ContainerSetCreate, containerSetId))
	if err != nil {
		return nil, l.handleError(err, "error publishing CREATE event", "name", containerSet.GetMeta().GetName(), "event", "ContainerCreate")
	}
	return &containersetsv1.CreateContainerSetResponse{
		ContainerSet: containerSet,
	}, nil
}

func (l *local) Delete(ctx context.Context, req *containersetsv1.DeleteContainerSetRequest, _ ...grpc.CallOption) (*containersetsv1.DeleteContainerSetResponse, error) {
	containerSet, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		return nil, l.handleError(err, "couldn't GET container from repo", "id", req.GetId())
	}
	containerSetId := containerSet.GetMeta().GetName()
	err = l.Repo().Delete(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	err = l.exchange.Publish(ctx, events.NewRequest(eventsv1.EventType_ContainerSetDelete, containerSetId))
	if err != nil {
		return nil, l.handleError(err, "error publishing DELETE event", "name", containerSet.GetMeta().GetName(), "event", "ContainerDelete")
	}
	return &containersetsv1.DeleteContainerSetResponse{
		Id: req.GetId(),
	}, nil
}

func (l *local) Update(ctx context.Context, req *containersetsv1.UpdateContainerSetRequest, _ ...grpc.CallOption) (*containersetsv1.UpdateContainerSetResponse, error) {
	updateMask := req.GetUpdateMask()
	updateContainerSet := req.GetContainerSet()
	existing, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		return nil, l.handleError(err, "couldn't GET container from repo", "name", updateContainerSet.GetMeta().GetName())
	}

	maskedUpdate, err := protoutils.ApplyFieldMaskToNewMessage(updateContainerSet, updateMask)
	if err != nil {
		return nil, err
	}
	proto.Merge(existing, maskedUpdate)

	err = l.Repo().Update(ctx, existing)
	if err != nil {
		return nil, l.handleError(err, "couldn't UPDATE container in repo", "name", existing.GetMeta().GetName())
	}
	err = l.exchange.Publish(ctx, events.NewRequest(eventsv1.EventType_ContainerSetUpdate, req.GetId()))
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
