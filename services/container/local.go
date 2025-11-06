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
	"github.com/amimof/blipblop/pkg/protoutils"
	"github.com/amimof/blipblop/pkg/repository"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type local struct {
	repo     repository.ContainerRepository
	mu       sync.Mutex
	exchange *events.Exchange
	logger   logger.Logger
}

var (
	_      containers.ContainerServiceClient = &local{}
	tracer                                   = otel.GetTracerProvider().Tracer("blipblop-server")
)

func (l *local) handleError(err error, msg string, keysAndValues ...any) error {
	def := []any{"error", err.Error()}
	def = append(def, keysAndValues...)
	l.logger.Error(msg, def...)
	if errors.Is(err, repository.ErrNotFound) {
		return status.Error(codes.NotFound, err.Error())
	}
	return status.Error(codes.Internal, err.Error())
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
	ctx, span := tracer.Start(ctx, "container.Get", trace.WithSpanKind(trace.SpanKindServer))
	span.SetAttributes(
		attribute.String("service", "Container"),
		attribute.String("container.id", req.GetId()),
	)
	defer span.End()

	// Validate request
	err := req.Validate()
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	// Get container from repo
	container, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		span.RecordError(err)
		return nil, l.handleError(err, "couldn't GET container from repo", "name", req.GetId())
	}

	span.SetAttributes(attribute.String("container.name", container.GetMeta().GetName()))

	return &containers.GetContainerResponse{
		Container: container,
	}, nil
}

func (l *local) List(ctx context.Context, req *containers.ListContainerRequest, _ ...grpc.CallOption) (*containers.ListContainerResponse, error) {
	ctx, span := tracer.Start(ctx, "container.List")
	defer span.End()

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
	ctx, span := tracer.Start(ctx, "container.Create")
	defer span.End()

	container := req.GetContainer()
	container.GetMeta().Created = timestamppb.Now()

	// Validate request
	err := req.ValidateAll()
	if err != nil {
		return nil, err
	}

	// Initialize status field if empty
	if container.GetStatus() == nil {
		container.Status = &containers.Status{}
	}

	containerID := container.GetMeta().GetName()

	// Chec if container already exists
	if existing, _ := l.Get(ctx, &containers.GetContainerRequest{Id: containerID}); existing != nil {
		return nil, fmt.Errorf("container %s already exists", container.GetMeta().GetName())
	}

	// Create container in repo
	err = l.Repo().Create(ctx, container)
	if err != nil {
		return nil, l.handleError(err, "couldn't CREATE container in repo", "name", containerID)
	}

	// Get the created container from repo
	container, err = l.Repo().Get(ctx, containerID)
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
	ctx, span := tracer.Start(ctx, "container.Delete")
	defer span.End()

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
	ctx, span := tracer.Start(ctx, "container.Kill")
	defer span.End()

	container, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}

	ev := eventsv1.EventType_ContainerStop
	if req.ForceKill {
		ev = eventsv1.EventType_ContainerKill
	}

	err = l.exchange.Publish(ctx, ev, events.NewEvent(ev, container))
	if err != nil {
		return nil, l.handleError(err, "error publishing STOP/KILL event", "name", req.GetId(), "event", ev.String())
	}
	return &containers.KillContainerResponse{
		Id: req.GetId(),
	}, nil
}

func (l *local) Start(ctx context.Context, req *containers.StartContainerRequest, _ ...grpc.CallOption) (*containers.StartContainerResponse, error) {
	ctx, span := tracer.Start(ctx, "container.Start")
	defer span.End()

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
	ctx, span := tracer.Start(ctx, "container.Patch")
	defer span.End()

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

func applyMaskedUpdate(dst, src *containers.Status, mask *fieldmaskpb.FieldMask) error {
	if mask == nil || len(mask.Paths) == 0 {
		return status.Error(codes.InvalidArgument, "update_mask is required")
	}

	for _, p := range mask.Paths {
		switch p {
		case "ip":
			if src.Ip == nil {
				continue
			}
			dst.Ip = src.Ip
		case "node":
			if src.Node == nil {
				continue
			}
			dst.Node = src.Node
		case "phase":
			if src.Phase == nil {
				continue
			}
			dst.Phase = src.Phase
		case "task.pid":
			if src.GetTask().Pid == nil {
				continue
			}
			fmt.Println(dst.GetTask())
			fmt.Println(src.GetTask())
			dst.GetTask().Pid = src.GetTask().Pid
		case "task.error":
			if src.GetTask().Error == nil {
				continue
			}
			dst.Task.Error = src.Task.Error
		default:
			return fmt.Errorf("unknown mask path %q", p)
		}
	}

	return nil
}

// UpdateStatus implements containers.ContainerServiceClient.
func (l *local) UpdateStatus(ctx context.Context, req *containers.UpdateStatusRequest, opts ...grpc.CallOption) (*containers.UpdateStatusResponse, error) {
	ctx, span := tracer.Start(ctx, "container.UpdateStatus")
	defer span.End()

	// Get the existing container before updating so we can compare specs
	existingContainer, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}

	// Apply mask safely
	base := proto.Clone(existingContainer.Status).(*containers.Status)
	if err := applyMaskedUpdate(base, req.Status, req.UpdateMask); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "bad mask: %v", err)
	}

	existingContainer.Status = base
	if err := l.Repo().Update(ctx, existingContainer); err != nil {
		return nil, err
	}

	return &containers.UpdateStatusResponse{
		Id: existingContainer.GetMeta().GetName(),
	}, nil
}

func (l *local) Update(ctx context.Context, req *containers.UpdateContainerRequest, _ ...grpc.CallOption) (*containers.UpdateContainerResponse, error) {
	ctx, span := tracer.Start(ctx, "container.Update")
	defer span.End()

	l.mu.Lock()
	defer l.mu.Unlock()

	// Validate request
	err := req.Validate()
	if err != nil {
		return nil, err
	}

	// Increase revision before updating
	updateContainer := req.GetContainer()
	updateContainer.Meta.Revision++

	// Get the existing container before updating so we can compare specs
	existingContainer, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}

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
	updVal := protoreflect.ValueOfMessage(updateContainer.GetConfig().ProtoReflect())
	newVal := protoreflect.ValueOfMessage(existingContainer.GetConfig().ProtoReflect())
	if !updVal.Equal(newVal) {
		l.logger.Debug("container was updated, emitting event to listeners", "event", "ContainerUpdate", "name", ctr.GetMeta().GetName())
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
