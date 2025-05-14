package container

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/amimof/blipblop/api/services/containers/v1"
	eventsv1 "github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/pkg/events"
	"github.com/amimof/blipblop/pkg/logger"
	"github.com/amimof/blipblop/pkg/protoutils"
	"github.com/amimof/blipblop/pkg/repository"
	jsonpatch "github.com/evanphx/json-patch/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
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

func removeReadOnlyFields(b []byte, fieldPaths []string) ([]byte, error) {
	var patchMap map[string]interface{}
	if err := json.Unmarshal(b, &patchMap); err != nil {
		return nil, err
	}

	// Remove excluded fields
	for _, fieldPath := range fieldPaths {
		parts := strings.Split(fieldPath, ".")
		removeNestedField(patchMap, parts)
	}

	patchJSON, err := json.Marshal(patchMap)
	if err != nil {
		return nil, err
	}

	mergedJSON, err := jsonpatch.MergePatch(b, patchJSON)
	if err != nil {
		return nil, err
	}

	return mergedJSON, nil
}

func removeNestedField(m map[string]interface{}, fields []string) {
	if len(fields) == 0 {
		return
	}

	key := fields[0]

	if len(fields) == 1 {
		delete(m, key)
		return
	}

	if nestedMap, ok := m[key].(map[string]interface{}); ok {
		removeNestedField(nestedMap, fields[1:])
	}
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

// Merge lists strategically using merge keys
func merge(base, patch *containers.Container) *containers.Container {
	merged := protoutils.StrategicMerge(base, patch,
		func(b, p *containers.Container) {
			if patch.Config == nil {
				return
			}
			b.Config.Envvars = protoutils.MergeSlices(b.Config.Envvars, p.Config.Envvars,
				func(e *containers.EnvVar) string {
					return e.Name
				},
				func(b, p *containers.EnvVar) *containers.EnvVar {
					if p.Value != "" {
						b.Value = p.Value
					}
					return b
				},
			)
		},
		func(b, p *containers.Container) {
			if patch.Config == nil {
				return
			}
			b.Config.PortMappings = protoutils.MergeSlices(b.Config.PortMappings, p.Config.PortMappings,
				func(e *containers.PortMapping) string {
					return e.Name
				},
				func(b, p *containers.PortMapping) *containers.PortMapping {
					if p.ContainerPort != 0 {
						b = p
					}
					return b
				},
			)
		},
		func(b, p *containers.Container) {
			if patch.Config == nil {
				return
			}
			b.Config.Mounts = protoutils.MergeSlices(b.Config.Mounts, p.Config.Mounts,
				func(e *containers.Mount) string {
					return e.Name
				},
				func(b, p *containers.Mount) *containers.Mount {
					return p
				},
			)
		},
		func(b, p *containers.Container) {
			if patch.Config == nil {
				return
			}
			b.Config.Args = protoutils.MergeSlices(b.Config.Args, p.Config.Args,
				func(e string) string {
					return e
				},
				func(b, p string) string {
					if p != "" {
						b = p
					}
					return b
				},
			)
		},
	)
	return merged
}

func (l *local) Get(ctx context.Context, req *containers.GetContainerRequest, _ ...grpc.CallOption) (*containers.GetContainerResponse, error) {
	// Validate request
	err := req.Validate()
	if err != nil {
		return nil, err
	}

	// Get container from repo
	container, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		return nil, l.handleError(err, "couldn't GET container from repo", "name", req.GetId())
	}
	return &containers.GetContainerResponse{
		Container: container,
	}, nil
}

func (l *local) List(ctx context.Context, req *containers.ListContainerRequest, _ ...grpc.CallOption) (*containers.ListContainerResponse, error) {
	// Validate request
	err := req.Validate()
	if err != nil {
		return nil, err
	}

	// Get containers from repo
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
	container.GetMeta().Created = timestamppb.Now()

	// Validate request
	err := req.ValidateAll()
	if err != nil {
		return nil, err
	}

	containerId := container.GetMeta().GetName()

	// Chec if container already exists
	if existing, _ := l.Get(ctx, &containers.GetContainerRequest{Id: containerId}); existing != nil {
		return nil, fmt.Errorf("container %s already exists", container.GetMeta().GetName())
	}

	// Create container in repo
	err = l.Repo().Create(ctx, container)
	if err != nil {
		return nil, l.handleError(err, "couldn't CREATE container in repo", "name", containerId)
	}

	// Get the created container from repo
	container, err = l.Repo().Get(ctx, containerId)
	if err != nil {
		return nil, err
	}

	// Publish event that container is created
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

	err = l.exchange.Publish(ctx, eventsv1.EventType_ContainerStart, events.NewEvent(eventsv1.EventType_ContainerStart, container))
	if err != nil {
		return nil, err
	}

	return &containers.StartContainerResponse{
		Id: req.GetId(),
	}, nil
}

func (l *local) Patch(ctx context.Context, req *containers.UpdateContainerRequest, _ ...grpc.CallOption) (*containers.UpdateContainerResponse, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Validate request
	err := req.Validate()
	if err != nil {
		return nil, err
	}

	updateContainer := req.GetContainer()

	// Get existing container from repo
	existing, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		return nil, l.handleError(err, "couldn't GET container from repo", "name", updateContainer.GetMeta().GetName())
	}

	// Generate field mask
	genFieldMask, err := protoutils.GenerateFieldMask(existing, updateContainer)
	if err != nil {
		return nil, err
	}

	// Handle partial update
	maskedUpdate, err := protoutils.ApplyFieldMaskToNewMessage(updateContainer, genFieldMask)
	if err != nil {
		return nil, err
	}

	// TODO: Handle errors
	updated := maskedUpdate.(*containers.Container)
	existing = merge(existing, updated)

	// Validate
	err = existing.Validate()
	if err != nil {
		return nil, err
	}

	// Update the container
	err = l.Repo().Update(ctx, existing)
	if err != nil {
		return nil, l.handleError(err, "couldn't PATCH container in repo", "name", existing.GetMeta().GetName())
	}

	// Retreive the container again so that we can include it in an event
	ctr, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}

	// Only publish if spec is updated
	if !proto.Equal(updateContainer.Config, ctr.Config) {
		err = l.exchange.Publish(ctx, eventsv1.EventType_ContainerPatch, events.NewEvent(eventsv1.EventType_ContainerPatch, ctr))
		if err != nil {
			return nil, l.handleError(err, "error publishing PATCH event", "name", existing.GetMeta().GetName(), "event", "ContainerUpdate")
		}
	}

	return &containers.UpdateContainerResponse{
		Container: existing,
	}, nil
}

func (l *local) Update(ctx context.Context, req *containers.UpdateContainerRequest, _ ...grpc.CallOption) (*containers.UpdateContainerResponse, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Validate request
	err := req.Validate()
	if err != nil {
		return nil, err
	}

	updateContainer := req.GetContainer()

	// Update the container
	err = l.Repo().Update(ctx, updateContainer)
	if err != nil {
		return nil, l.handleError(err, "couldn't UPDATE container in repo", "name", updateContainer.GetMeta().GetName())
	}

	// Retreive the container again so that we can include it in an event
	ctr, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}

	// Only publish if spec is updated
	if !proto.Equal(updateContainer.Config, ctr.Config) {
		err = l.exchange.Publish(ctx, eventsv1.EventType_ContainerUpdate, events.NewEvent(eventsv1.EventType_ContainerUpdate, ctr))
		if err != nil {
			return nil, l.handleError(err, "error publishing UPDATE event", "name", ctr.GetMeta().GetName(), "event", "ContainerUpdate")
		}
	}

	return &containers.UpdateContainerResponse{
		Container: ctr,
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
