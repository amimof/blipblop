package task

import (
	"context"
	"errors"
	"fmt"
	"sync"

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

	"github.com/amimof/voiyd/pkg/events"
	"github.com/amimof/voiyd/pkg/logger"
	"github.com/amimof/voiyd/pkg/protoutils"
	"github.com/amimof/voiyd/pkg/repository"

	eventsv1 "github.com/amimof/voiyd/api/services/events/v1"
	tasksv1 "github.com/amimof/voiyd/api/services/tasks/v1"
)

type local struct {
	repo     repository.TaskRepository
	mu       sync.Mutex
	exchange *events.Exchange
	logger   logger.Logger
}

var (
	_      tasksv1.TaskServiceClient = &local{}
	tracer                           = otel.GetTracerProvider().Tracer("voiyd-server")
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
func merge(base, patch *tasksv1.Task) *tasksv1.Task {
	merged := protoutils.StrategicMerge(base, patch,
		func(b, p *tasksv1.Task) {
			if patch.Config == nil {
				return
			}
			b.Config.Envvars = protoutils.MergeSlices(b.Config.Envvars, p.Config.Envvars,
				func(e *tasksv1.EnvVar) string {
					return e.Name
				},
				func(b, p *tasksv1.EnvVar) *tasksv1.EnvVar {
					if p.Value != "" {
						b.Value = p.Value
					}
					return b
				},
			)
		},
		func(b, p *tasksv1.Task) {
			if patch.Config == nil {
				return
			}
			b.Config.PortMappings = protoutils.MergeSlices(b.Config.PortMappings, p.Config.PortMappings,
				func(e *tasksv1.PortMapping) string {
					return e.Name
				},
				func(b, p *tasksv1.PortMapping) *tasksv1.PortMapping {
					if p.TargetPort != 0 {
						b = p
					}
					return b
				},
			)
		},
		func(b, p *tasksv1.Task) {
			if patch.Config == nil {
				return
			}
			b.Config.Mounts = protoutils.MergeSlices(b.Config.Mounts, p.Config.Mounts,
				func(e *tasksv1.Mount) string {
					return e.Name
				},
				func(b, p *tasksv1.Mount) *tasksv1.Mount {
					return p
				},
			)
		},
		func(b, p *tasksv1.Task) {
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

func (l *local) Get(ctx context.Context, req *tasksv1.GetRequest, _ ...grpc.CallOption) (*tasksv1.GetResponse, error) {
	ctx, span := tracer.Start(ctx, "container.Get", trace.WithSpanKind(trace.SpanKindServer))
	span.SetAttributes(
		attribute.String("service", "Task"),
		attribute.String("container.id", req.GetId()),
	)
	defer span.End()

	// Get container from repo
	container, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		span.RecordError(err)
		return nil, l.handleError(err, "couldn't GET container from repo", "name", req.GetId())
	}

	span.SetAttributes(attribute.String("container.name", container.GetMeta().GetName()))

	return &tasksv1.GetResponse{
		Task: container,
	}, nil
}

func (l *local) List(ctx context.Context, req *tasksv1.ListRequest, _ ...grpc.CallOption) (*tasksv1.ListResponse, error) {
	ctx, span := tracer.Start(ctx, "container.List")
	defer span.End()

	// Validate request

	// Get containers from repo
	ctrs, err := l.Repo().List(ctx, req.GetSelector())
	if err != nil {
		return nil, l.handleError(err, "couldn't LIST containers from repo")
	}
	return &tasksv1.ListResponse{
		Tasks: ctrs,
	}, nil
}

func (l *local) Create(ctx context.Context, req *tasksv1.CreateRequest, _ ...grpc.CallOption) (*tasksv1.CreateResponse, error) {
	ctx, span := tracer.Start(ctx, "container.Create")
	defer span.End()

	container := req.GetTask()
	container.GetMeta().Created = timestamppb.Now()
	container.GetMeta().Updated = timestamppb.Now()
	container.GetMeta().Revision = 1

	// Initialize status field if empty
	if container.GetStatus() == nil {
		container.Status = &tasksv1.Status{}
	}

	containerID := container.GetMeta().GetName()

	// Check if container already exists
	if existing, _ := l.Get(ctx, &tasksv1.GetRequest{Id: containerID}); existing != nil {
		return nil, fmt.Errorf("container %s already exists", container.GetMeta().GetName())
	}

	// Create container in repo
	err := l.Repo().Create(ctx, container)
	if err != nil {
		return nil, l.handleError(err, "couldn't CREATE container in repo", "name", containerID)
	}

	// Get the created container from repo
	container, err = l.Repo().Get(ctx, containerID)
	if err != nil {
		return nil, err
	}

	// Publish event that container is created
	err = l.exchange.Publish(ctx, eventsv1.EventType_TaskCreate, events.NewEvent(eventsv1.EventType_TaskCreate, container))
	if err != nil {
		return nil, l.handleError(err, "error publishing CREATE event", "name", container.GetMeta().GetName(), "event", "TaskCreate")
	}

	return &tasksv1.CreateResponse{
		Task: container,
	}, nil
}

// Delete publishes a delete request and the subscribers are responsible for deleting resources.
// Once they do, they will update there resource with the status Deleted
func (l *local) Delete(ctx context.Context, req *tasksv1.DeleteRequest, _ ...grpc.CallOption) (*tasksv1.DeleteResponse, error) {
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
	// err = l.exchange.Publish(ctx, events.NewRequest(eventsv1.EventType_TaskDelete, container))
	err = l.exchange.Publish(ctx, eventsv1.EventType_TaskDelete, events.NewEvent(eventsv1.EventType_TaskDelete, container))
	if err != nil {
		return nil, l.handleError(err, "error publishing DELETE event", "name", container.GetMeta().GetName(), "event", "TaskDelete")
	}
	return &tasksv1.DeleteResponse{
		Id: req.GetId(),
	}, nil
}

func (l *local) Kill(ctx context.Context, req *tasksv1.KillRequest, _ ...grpc.CallOption) (*tasksv1.KillResponse, error) {
	ctx, span := tracer.Start(ctx, "container.Kill")
	defer span.End()

	container, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}

	ev := eventsv1.EventType_TaskStop
	if req.ForceKill {
		ev = eventsv1.EventType_TaskKill
	}

	err = l.exchange.Publish(ctx, ev, events.NewEvent(ev, container))
	if err != nil {
		return nil, l.handleError(err, "error publishing STOP/KILL event", "name", req.GetId(), "event", ev.String())
	}
	return &tasksv1.KillResponse{
		Id: req.GetId(),
	}, nil
}

func (l *local) Start(ctx context.Context, req *tasksv1.StartRequest, _ ...grpc.CallOption) (*tasksv1.StartResponse, error) {
	ctx, span := tracer.Start(ctx, "container.Start")
	defer span.End()

	container, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}

	err = l.exchange.Publish(ctx, eventsv1.EventType_TaskStart, events.NewEvent(eventsv1.EventType_TaskStart, container))
	if err != nil {
		return nil, err
	}

	return &tasksv1.StartResponse{
		Id: req.GetId(),
	}, nil
}

func (l *local) Patch(ctx context.Context, req *tasksv1.PatchRequest, _ ...grpc.CallOption) (*tasksv1.PatchResponse, error) {
	ctx, span := tracer.Start(ctx, "container.Patch")
	defer span.End()

	l.mu.Lock()
	defer l.mu.Unlock()

	// Validate request

	updateTask := req.GetTask()

	// Get existing container from repo
	existing, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		return nil, l.handleError(err, "couldn't GET container from repo", "name", updateTask.GetMeta().GetName())
	}

	// Generate field mask
	genFieldMask, err := protoutils.GenerateFieldMask(existing, updateTask)
	if err != nil {
		return nil, err
	}

	// Handle partial update
	maskedUpdate, err := protoutils.ApplyFieldMaskToNewMessage(updateTask, genFieldMask)
	if err != nil {
		return nil, err
	}

	// TODO: Handle errors
	updated := maskedUpdate.(*tasksv1.Task)
	existing = merge(existing, updated)

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
	if !proto.Equal(updateTask.Config, ctr.Config) {
		err = l.exchange.Publish(ctx, eventsv1.EventType_TaskPatch, events.NewEvent(eventsv1.EventType_TaskPatch, ctr))
		if err != nil {
			return nil, l.handleError(err, "error publishing PATCH event", "name", existing.GetMeta().GetName(), "event", "TaskUpdate")
		}
	}

	return &tasksv1.PatchResponse{
		Task: existing,
	}, nil
}

func applyMaskedUpdate(dst, src *tasksv1.Status, mask *fieldmaskpb.FieldMask) error {
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
		case "id":
			if src.Id == nil {
				continue
			}
			dst.Id = src.Id
		case "status":
			if src.Status == nil {
				continue
			}
			dst.Status = src.Status
		case "task.pid":
			if src.GetTask().Pid == nil {
				continue
			}
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

// UpdateStatus implements containers.TaskServiceClient.
func (l *local) UpdateStatus(ctx context.Context, req *tasksv1.UpdateStatusRequest, opts ...grpc.CallOption) (*tasksv1.UpdateStatusResponse, error) {
	ctx, span := tracer.Start(ctx, "container.UpdateStatus")
	defer span.End()

	// Get the existing container before updating so we can compare specs
	existingTask, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}

	// Apply mask safely
	base := proto.Clone(existingTask.Status).(*tasksv1.Status)
	if err := applyMaskedUpdate(base, req.Status, req.UpdateMask); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "bad mask: %v", err)
	}

	existingTask.Status = base
	if err := l.Repo().Update(ctx, existingTask); err != nil {
		return nil, err
	}

	return &tasksv1.UpdateStatusResponse{
		Id: existingTask.GetMeta().GetName(),
	}, nil
}

func (l *local) Update(ctx context.Context, req *tasksv1.UpdateRequest, _ ...grpc.CallOption) (*tasksv1.UpdateResponse, error) {
	ctx, span := tracer.Start(ctx, "container.Update")
	defer span.End()

	l.mu.Lock()
	defer l.mu.Unlock()

	updateTask := req.GetTask()

	// Get the existing container before updating so we can compare specs
	existingTask, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}

	// Ignore fields
	updateTask.Status = existingTask.Status
	updateTask.GetMeta().Updated = existingTask.Meta.Updated
	updateTask.GetMeta().Created = existingTask.Meta.Created
	updateTask.GetMeta().Revision = existingTask.Meta.Revision

	updVal := protoreflect.ValueOfMessage(updateTask.GetConfig().ProtoReflect())
	newVal := protoreflect.ValueOfMessage(existingTask.GetConfig().ProtoReflect())

	// Only update metadata fields if spec is updated
	if !updVal.Equal(newVal) {
		updateTask.Meta.Revision++
		updateTask.Meta.Updated = timestamppb.Now()
	}

	// Update the container
	err = l.Repo().Update(ctx, updateTask)
	if err != nil {
		return nil, l.handleError(err, "couldn't UPDATE container in repo", "name", updateTask.GetMeta().GetName())
	}

	// Retreive the container again so that we can include it in an event
	ctr, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}

	// Only publish if spec is updated
	if !updVal.Equal(newVal) {

		l.logger.Debug("container was updated, emitting event to listeners", "event", "TaskUpdate", "name", ctr.GetMeta().GetName(), "revision", updateTask.GetMeta().GetRevision())
		err = l.exchange.Publish(ctx, eventsv1.EventType_TaskUpdate, events.NewEvent(eventsv1.EventType_TaskUpdate, ctr))
		if err != nil {
			return nil, l.handleError(err, "error publishing UPDATE event", "name", ctr.GetMeta().GetName(), "event", "TaskUpdate")
		}
	}

	return &tasksv1.UpdateResponse{
		Task: ctr,
	}, nil
}

func (l *local) Repo() repository.TaskRepository {
	if l.repo != nil {
		return l.repo
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return repository.NewTaskInMemRepo()
}
