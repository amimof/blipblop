package container

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/amimof/blipblop/api/services/containers/v1"
	eventsv1 "github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/pkg/events"
	"github.com/amimof/blipblop/pkg/eventsv2"
	"github.com/amimof/blipblop/pkg/logger"
	"github.com/amimof/blipblop/pkg/protoutils"
	"github.com/amimof/blipblop/pkg/repository"
	jsonpatch "github.com/evanphx/json-patch"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type local struct {
	repo     repository.ContainerRepository
	mu       sync.Mutex
	exchange *eventsv2.Exchange
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
	ctrs, err := l.Repo().List(ctx, req.GetSelector())
	if err != nil {
		return nil, l.handleError(err, "couldn't LIST containers from repo")
	}
	return &containers.ListContainerResponse{
		Containers: ctrs,
	}, nil
}

func (l *local) Create(ctx context.Context, req *containers.CreateContainerRequest, _ ...grpc.CallOption) (*containers.CreateContainerResponse, error) {
	container := req.GetContainer()
	containerId := container.GetMeta().GetName()

	if existing, _ := l.Repo().Get(ctx, container.GetMeta().GetName()); existing != nil {
		return nil, fmt.Errorf("container %s already exists", container.GetMeta().GetName())
	}

	container.GetMeta().Created = timestamppb.Now()

	err := req.Validate()
	if err != nil {
		return nil, err
	}

	err = l.Repo().Create(ctx, container)
	if err != nil {
		return nil, l.handleError(err, "couldn't CREATE container in repo", "name", containerId)
	}

	container, err = l.Repo().Get(ctx, containerId)
	if err != nil {
		return nil, err
	}

	// err = l.exchange.Publish(ctx, events.NewRequest(eventsv1.EventType_ContainerCreate, container))
	err = l.exchange.Publish(ctx, eventsv1.EventType_ContainerCreate, events.NewEvent(eventsv1.EventType_ContainerCreate, container))
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
	err = l.Repo().Delete(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	// err = l.exchange.Publish(ctx, events.NewRequest(eventsv1.EventType_ContainerDelete, container))
	err = l.exchange.Publish(ctx, eventsv1.EventType_ContainerDelete, events.NewEvent(eventsv1.EventType_ContainerDelete, container))
	if err != nil {
		return nil, l.handleError(err, "error publishing DELETE event", "name", container.GetMeta().GetName(), "event", "ContainerDelete")
	}
	return &containers.DeleteContainerResponse{
		Id: req.GetId(),
	}, nil
}

func (l *local) Kill(ctx context.Context, req *containers.KillContainerRequest, _ ...grpc.CallOption) (*containers.KillContainerResponse, error) {
	container, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	// err = l.exchange.Publish(ctx, events.NewRequest(eventsv1.EventType_ContainerKill, container))
	err = l.exchange.Publish(ctx, eventsv1.EventType_ContainerKill, events.NewEvent(eventsv1.EventType_ContainerKill, container))
	if err != nil {
		return nil, l.handleError(err, "error publishing KILL event", "name", req.GetId(), "event", "ContainerKill")
	}
	return &containers.KillContainerResponse{
		Id: req.GetId(),
	}, nil
}

func (l *local) Start(ctx context.Context, req *containers.StartContainerRequest, _ ...grpc.CallOption) (*containers.StartContainerResponse, error) {
	container, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	// err = l.exchange.Publish(ctx, events.NewRequest(eventsv1.EventType_ContainerStart, container))
	err = l.exchange.Publish(ctx, eventsv1.EventType_ContainerStart, events.NewEvent(eventsv1.EventType_ContainerStart, container))
	if err != nil {
		return nil, err
	}
	return &containers.StartContainerResponse{
		Id: req.GetId(),
	}, nil
}

func removeImmutableFields(dst *containers.Container) error {
	if dst.Meta == nil {
		return nil
	}
	if dst.Meta.Name != "" {
		dst.Meta.Name = ""
	}

	if dst.Meta.Created != nil {
		dst.Meta.Created = nil
	}

	return nil
}

// mergePatch uses json to apply the patch to the target
func mergePatch(target, patch *containers.Container) (*containers.Container, error) {
	targetb, err := json.Marshal(target)
	if err != nil {
		return nil, err
	}

	patchb, err := json.Marshal(patch)
	if err != nil {
		return nil, err
	}

	b, err := jsonpatch.MergePatch(targetb, patchb)
	if err != nil {
		return nil, err
	}

	var c containers.Container
	err = json.Unmarshal(b, &c)
	if err != nil {
		return nil, err
	}

	return &c, nil
}

func (l *local) Update(ctx context.Context, req *containers.UpdateContainerRequest, _ ...grpc.CallOption) (*containers.UpdateContainerResponse, error) {
	updateMask := req.GetUpdateMask()
	updateContainer := req.GetContainer()

	// Get existing container from repo
	existing, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		return nil, l.handleError(err, "couldn't GET container from repo", "name", updateContainer.GetMeta().GetName())
	}

	// Filter out immutable fields before we update
	err = removeImmutableFields(updateContainer)
	if err != nil {
		return nil, err
	}

	// Handle partial update
	maskedUpdate, err := protoutils.ApplyFieldMaskToNewMessage(updateContainer, updateMask)
	if err != nil {
		return nil, err
	}

	// Apply the patch
	updated, err := mergePatch(existing, maskedUpdate.(*containers.Container))
	if err != nil {
		return nil, err
	}

	// Update updated field
	updated.GetMeta().Updated = timestamppb.Now()

	// Validate
	err = updated.Validate()
	if err != nil {
		return nil, err
	}

	// Update the container
	err = l.Repo().Update(ctx, updated)
	if err != nil {
		return nil, l.handleError(err, "couldn't UPDATE container in repo", "name", existing.GetMeta().GetName())
	}

	// Retreive the container again so that we can include it in an event
	ctr, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}

	// Only publish if spec is updated
	if !proto.Equal(updated.Config, existing.Config) {
		// err = l.exchange.Publish(ctx, events.NewRequest(eventsv1.EventType_ContainerUpdate, ctr))
		err = l.exchange.Publish(ctx, eventsv1.EventType_ContainerUpdate, events.NewEvent(eventsv1.EventType_ContainerUpdate, ctr))
		if err != nil {
			return nil, l.handleError(err, "error publishing UPDATE event", "name", existing.GetMeta().GetName(), "event", "ContainerUpdate")
		}
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
