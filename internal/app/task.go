package app

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/amimof/voiyd/internal/domain"
	"github.com/amimof/voiyd/pkg/condition"
	"github.com/amimof/voiyd/pkg/errs"
	"github.com/amimof/voiyd/pkg/events"
	"github.com/amimof/voiyd/pkg/keys"
	"github.com/amimof/voiyd/pkg/logger"
	"github.com/amimof/voiyd/pkg/protoutils"
	"github.com/amimof/voiyd/pkg/repository"

	tasksv1 "github.com/amimof/voiyd/api/services/tasks/v1"
	typesv1 "github.com/amimof/voiyd/api/types/v1"
)

type TaskService struct {
	Repo      *repository.Repo[*tasksv1.Task]
	mu        sync.Mutex
	Exchange  *events.Exchange
	Logger    logger.Logger
	Scheduler Scheduler
	Manager   LeaseStore
}

func (l *TaskService) handleError(err error, msg string, keysAndValues ...any) error {
	def := []any{"error", err.Error()}
	def = append(def, keysAndValues...)
	l.Logger.Error(msg, def...)
	if errors.Is(err, repository.ErrNotFound) {
		return status.Error(codes.NotFound, err.Error())
	}
	return status.Error(codes.Internal, err.Error())
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
		case "reason":
			if src.Reason == nil {
				continue
			}
			dst.Reason = src.Reason
		case "pid":
			if src.Pid == nil {
				continue
			}
			dst.Pid = src.Pid
		case "conditions":
			if src.Conditions == nil {
				continue
			}
			// Merge conditions intelligently, keeping newest based on timestamp
			dst.Conditions = mergeConditions(dst.Conditions, src.Conditions, &logger.ConsoleLogger{})
		default:
			return fmt.Errorf("unknown mask path %q", p)
		}
	}

	return nil
}

// mergeConditions merges incoming conditions into existing conditions,
// keeping the newest version of each condition type based on LastTransitionTime.
// This prevents stale condition updates from overwriting newer ones.
func mergeConditions(existing, incoming []*typesv1.Condition, logger logger.Logger) []*typesv1.Condition {
	return protoutils.MergeSlices(
		existing,
		incoming,
		func(c *typesv1.Condition) string {
			// Use condition Type as the merge key
			return c.Type.GetValue()
		},
		func(existingCond, incomingCond *typesv1.Condition) *typesv1.Condition {
			// Determine which condition to keep based on timestamp
			if incomingCond.LastTransitionTime == nil {
				// Incoming has no timestamp, keep existing
				logger.Debug("incoming condition has no timestamp, keeping existing",
					"type", existingCond.Type.GetValue())
				return existingCond
			}

			if existingCond.LastTransitionTime == nil {
				// Existing has no timestamp, accept incoming
				return incomingCond
			}

			// Both have timestamps - compare them
			if incomingCond.LastTransitionTime.AsTime().After(existingCond.LastTransitionTime.AsTime()) {
				// Incoming is newer, use it
				return incomingCond
			}

			// Incoming is stale, keep existing
			logger.Debug("rejecting stale condition",
				"type", incomingCond.Type.GetValue(),
				"incoming_time", incomingCond.LastTransitionTime.AsTime(),
				"existing_time", existingCond.LastTransitionTime.AsTime())
			return existingCond
		},
	)
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

func (l *TaskService) Get(ctx context.Context, id keys.ID) (*tasksv1.Task, error) {
	ctx, span := tracer.Start(ctx, "task.Get", trace.WithSpanKind(trace.SpanKindServer))
	defer span.End()

	return l.Repo.Get(ctx, id)
}

func (l *TaskService) List(ctx context.Context, limit int32) ([]*tasksv1.Task, error) {
	ctx, span := tracer.Start(ctx, "task.List")
	defer span.End()

	// Get tasks from repo
	return l.Repo.List(ctx, limit)
}

func (l *TaskService) Create(ctx context.Context, task *tasksv1.Task) (*tasksv1.Task, error) {
	ctx, span := tracer.Start(ctx, "task.Create")
	defer span.End()

	l.mu.Lock()
	defer l.mu.Unlock()

	// Create task in repo
	newTask, err := l.Repo.Create(ctx, task)
	if err != nil {
		return nil, l.handleError(err, "error creating task", "name", newTask.GetMeta().GetName())
	}

	// Publish event that task is created
	err = l.Exchange.Forward(ctx, events.NewEvent(events.TaskCreate, task))
	if err != nil {
		return nil, l.handleError(err, "error publishing task create event", "name", newTask.GetMeta().GetName())
	}

	return newTask, nil
}

// Delete publishes a delete request and the subscribers are responsible for deleting resources.
// Once they do, they will update there resource with the status Deleted
func (l *TaskService) Delete(ctx context.Context, id keys.ID) error {
	ctx, span := tracer.Start(ctx, "task.Delete")
	defer span.End()

	l.mu.Lock()
	defer l.mu.Unlock()

	task, err := l.Repo.Get(ctx, id)
	if err != nil {
		return err
	}

	err = l.Repo.Delete(ctx, id)
	if err != nil {
		return err
	}

	err = l.Exchange.Forward(ctx, events.NewEvent(events.TaskDelete, task))
	if err != nil {
		return l.handleError(err, "error publishing task delete event", "name", task.GetMeta().GetName())
	}

	return nil
}

func (l *TaskService) Kill(ctx context.Context, id keys.ID) error {
	ctx, span := tracer.Start(ctx, "task.Kill")
	defer span.End()

	task, err := l.Repo.Get(ctx, id)
	if err != nil {
		return err
	}

	lease, err := l.Manager.Get(ctx, ResourceID(task.GetMeta().GetUid()))
	if err != nil {
		if errs.IsNotFound(err) {
			return nil
		}
		return err
	}

	err = l.Scheduler.Kill(ctx, task, lease.GetConfig().GetNodeId())
	if err != nil {
		return err
	}

	return nil
}

func (l *TaskService) Stop(ctx context.Context, id keys.ID) error {
	ctx, span := tracer.Start(ctx, "task.Kill")
	defer span.End()

	task, err := l.Repo.Get(ctx, id)
	if err != nil {
		return err
	}

	lease, err := l.Manager.Get(ctx, ResourceID(task.GetMeta().GetUid()))
	if err != nil {
		if errs.IsNotFound(err) {
			return nil
		}
		return err
	}

	err = l.Scheduler.Stop(ctx, task, lease.GetConfig().GetNodeId())
	if err != nil {
		return err
	}

	return nil
}

func (l *TaskService) Start(ctx context.Context, id keys.ID) error {
	ctx, span := tracer.Start(ctx, "task.Start")
	defer span.End()

	task, err := l.Repo.Get(ctx, id)
	if err != nil {
		return err
	}

	n, err := l.Scheduler.Start(ctx, task)
	if err != nil {
		return err
	}

	err = l.UpdateStatus(
		ctx,
		id,
		&tasksv1.Status{
			Phase: wrapperspb.String(string(condition.ReasonScheduled)),
			Node:  wrapperspb.String(n.GetMeta().GetName()),
		},
		"phase",
		"node",
	)
	if err != nil {
		return err
	}

	return nil
}

func (l *TaskService) Patch(ctx context.Context, id keys.ID, patch *tasksv1.Task) error {
	ctx, span := tracer.Start(ctx, "task.Patch")
	defer span.End()

	l.mu.Lock()
	defer l.mu.Unlock()

	// Get existing task from repo
	existing, err := l.Repo.Get(ctx, id)
	if err != nil {
		return l.handleError(err, "error getting task", "name", patch.GetMeta().GetName())
	}

	// Generate field mask
	genFieldMask, err := protoutils.GenerateFieldMask(existing, patch)
	if err != nil {
		return err
	}

	// Handle partial update
	maskedUpdate, err := protoutils.ApplyFieldMaskToNewMessage(patch, genFieldMask)
	if err != nil {
		return err
	}

	// TODO: Handle errors
	updated := maskedUpdate.(*tasksv1.Task)
	existing = merge(existing, updated)

	// Update the task
	task, err := l.Repo.Update(ctx, id, existing)
	if err != nil {
		return l.handleError(err, "error updating task", "name", existing.GetMeta().GetName())
	}

	changed, err := protoutils.SpecEqual(existing.GetConfig(), task.GetConfig())
	if err != nil {
		return err
	}

	// Only publish if spec is updated
	if changed {
		err = l.Exchange.Forward(ctx, events.NewEvent(events.TaskPatch, task))
		if err != nil {
			return l.handleError(err, "error publishing task patch event", "name", existing.GetMeta().GetName())
		}
	}

	return nil
}

// UpdateStatus implements tasks.TaskServiceClient.
func (l *TaskService) UpdateStatus(ctx context.Context, id keys.ID, st *tasksv1.Status, mask ...string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	ctx, span := tracer.Start(ctx, "task.UpdateStatus")
	defer span.End()

	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if res, ok := md["x-voiyd-node-uid"]; ok && len(res) > 0 {
			if isHolder, _ := l.Manager.IsHolder(ctx, ResourceID(id.String()), HolderID(res[0])); !isHolder {
				if !isHolder {
					return domain.ErrNotHolder
				}
			}
		}
	}

	// Get the existing task before updating so we can compare specs
	existingTask, err := l.Repo.Get(ctx, id)
	if err != nil {
		return err
	}

	// Apply mask safely
	base := proto.Clone(existingTask.Status).(*tasksv1.Status)
	if err := applyMaskedUpdate(base, st, &fieldmaskpb.FieldMask{Paths: mask}); err != nil {
		return status.Errorf(codes.InvalidArgument, "bad mask: %v", err)
	}

	existingTask.Status = base

	if _, err := l.Repo.Update(ctx, id, existingTask); err != nil {
		return err
	}

	return nil
}

func (l *TaskService) Update(ctx context.Context, id keys.ID, task *tasksv1.Task) error {
	ctx, span := tracer.Start(ctx, "task.Update")
	defer span.End()

	l.mu.Lock()
	defer l.mu.Unlock()

	// Get the existing task before updating so we can compare specs
	existingTask, err := l.Repo.Get(ctx, id)
	if err != nil {
		return err
	}

	// Update the task
	updated, err := l.Repo.Update(ctx, id, task)
	if err != nil {
		return l.handleError(err, "error updating task", "name", task.GetMeta().GetName())
	}

	changed, err := protoutils.SpecEqual(existingTask.GetConfig(), updated.GetConfig())
	if err != nil {
		return err
	}

	// Only publish if spec is updated
	if changed {
		l.Logger.Debug("task was updated, emitting event to listeners", "event", "TaskUpdate", "name", updated.GetMeta().GetName())
		err = l.Exchange.Forward(ctx, events.NewEvent(events.TaskUpdate, updated))
		if err != nil {
			return l.handleError(err, "error publishing task update event", "name", updated.GetMeta().GetName())
		}
	}

	return nil
}

func (l *TaskService) Condition(ctx context.Context, id keys.ID, req *typesv1.ConditionRequest) error {
	st := &tasksv1.Status{
		Conditions: req.GetConditions(),
	}

	err := l.UpdateStatus(ctx, id, st, "conditions")
	if err != nil {
		return err
	}

	err = l.Exchange.Publish(ctx, events.NewEvent(events.ConditionReported, req))
	if err != nil {
		return err
	}
	return nil
}
