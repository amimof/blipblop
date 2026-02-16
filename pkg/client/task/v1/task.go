package v1

import (
	"context"

	"go.opentelemetry.io/otel"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/amimof/voiyd/pkg/keys"
	"github.com/amimof/voiyd/pkg/labels"
	"github.com/amimof/voiyd/pkg/util"
	"github.com/amimof/voiyd/services/task"

	tasksv1 "github.com/amimof/voiyd/api/services/tasks/v1"
	typesv1 "github.com/amimof/voiyd/api/types/v1"
)

const (
	TaskHealthHealthy   = "healthy"
	TaskHealthUnhealthy = "unhealthy"
)

type CreateOption func(c *clientV1)

func WithEmitLabels(l labels.Label) CreateOption {
	return func(c *clientV1) {
		c.emitLabels = l
	}
}

func WithClient(client tasksv1.TaskServiceClient) CreateOption {
	return func(c *clientV1) {
		c.Client = client
	}
}

type ClientV1 interface {
	Status() StatusClientV1
	Kill(context.Context, string) error
	Stop(context.Context, string) error
	Start(context.Context, string) error
	Create(context.Context, *tasksv1.Task, ...CreateOption) error
	Update(context.Context, string, *tasksv1.Task) error
	Patch(context.Context, string, *tasksv1.Task) error
	Get(context.Context, string) (*tasksv1.Task, error)
	Delete(context.Context, string) error
	List(context.Context, ...labels.Label) ([]*tasksv1.Task, error)
	Condition(context.Context, string, ...*typesv1.Condition) error
}

type StatusClientV1 interface {
	Update(context.Context, string, *tasksv1.Status, ...string) error
	SetPhase(context.Context, string, string) error
	SetNode(context.Context, string, string) error
	SetID(context.Context, string, string) error
	SetPid(context.Context, string, uint32) error
	SetReason(context.Context, string, string) error
}

type clientV1 struct {
	Client     tasksv1.TaskServiceClient
	emitLabels labels.Label
	id         string
}

type statusV1 struct {
	client tasksv1.TaskServiceClient
}

// SetID implements [StatusClientV1].
func (c *statusV1) SetID(ctx context.Context, id string, i string) error {
	return c.Update(ctx, id, &tasksv1.Status{Id: wrapperspb.String(i)}, "id")
}

// SetNode implements [StatusClientV1].
func (c *statusV1) SetNode(ctx context.Context, id string, node string) error {
	return c.Update(ctx, id, &tasksv1.Status{Node: wrapperspb.String(node)}, "node")
}

// SetPhase implements [StatusClientV1].
func (c *statusV1) SetPhase(ctx context.Context, id string, phase string) error {
	return c.Update(ctx, id, &tasksv1.Status{Phase: wrapperspb.String(phase)}, "phase")
}

// SetPid implements [StatusClientV1].
func (c *statusV1) SetPid(ctx context.Context, id string, pid uint32) error {
	return c.Update(ctx, id, &tasksv1.Status{Pid: wrapperspb.UInt32(pid)}, "pid")
}

// SetReason implements [StatusClientV1].
func (c *statusV1) SetReason(ctx context.Context, id string, reason string) error {
	return c.Update(ctx, id, &tasksv1.Status{Reason: wrapperspb.String(reason)}, "reason")
}

func (c *clientV1) Status() StatusClientV1 {
	return &statusV1{
		client: c.Client,
	}
}

func (c *statusV1) Update(ctx context.Context, id string, status *tasksv1.Status, path ...string) error {
	// Construct field mask
	mask := &fieldmaskpb.FieldMask{
		Paths: path,
	}

	uid, err := keys.ParseStr(id)
	if err != nil {
		return err
	}

	req := &tasksv1.UpdateStatusRequest{
		Name:       uid.NameStr(),
		Uid:        uid.UUIDStr(),
		UpdateMask: mask,
		Status:     status,
	}

	_, err = c.client.UpdateStatus(ctx, req)
	if err != nil {
		return err
	}

	return nil
}

func (c *clientV1) Kill(ctx context.Context, id string) error {
	tracer := otel.Tracer("client-v1")
	ctx, span := tracer.Start(ctx, "client.task.Kill")
	defer span.End()

	uid, err := keys.ParseStr(id)
	if err != nil {
		return err
	}

	ctx = metadata.AppendToOutgoingContext(ctx, "voiyd_client_id", c.id)
	_, err = c.Client.Kill(ctx, &tasksv1.KillRequest{Uid: uid.UUIDStr(), Name: uid.NameStr(), ForceKill: true})
	if err != nil {
		return err
	}
	return err
}

func (c *clientV1) Stop(ctx context.Context, id string) error {
	tracer := otel.Tracer("client-v1")
	ctx, span := tracer.Start(ctx, "client.task.Start")
	defer span.End()

	uid, err := keys.ParseStr(id)
	if err != nil {
		return err
	}

	ctx = metadata.AppendToOutgoingContext(ctx, "voiyd_client_id", c.id)
	_, err = c.Client.Kill(ctx, &tasksv1.KillRequest{Uid: uid.UUIDStr(), Name: uid.NameStr(), ForceKill: false})
	if err != nil {
		return err
	}
	return err
}

func (c *clientV1) Start(ctx context.Context, id string) error {
	tracer := otel.Tracer("client-v1")
	ctx, span := tracer.Start(ctx, "client.task.Start")
	defer span.End()

	uid, err := keys.ParseStr(id)
	if err != nil {
		return err
	}

	ctx = metadata.AppendToOutgoingContext(ctx, "voiyd_client_id", c.id)
	_, err = c.Client.Start(ctx, &tasksv1.StartRequest{Uid: uid.UUIDStr(), Name: uid.NameStr()})
	if err != nil {
		return err
	}

	return err
}

func (c *clientV1) Create(ctx context.Context, ctr *tasksv1.Task, opts ...CreateOption) error {
	tracer := otel.Tracer("client-v1")
	ctx, span := tracer.Start(ctx, "client.task.Update")
	defer span.End()

	if ctr.Version == "" {
		ctr.Version = task.Version
	}

	for _, opt := range opts {
		opt(c)
	}

	ctx = metadata.AppendToOutgoingContext(ctx, "voiyd_client_id", c.id)
	_, err := c.Client.Create(ctx, &tasksv1.CreateRequest{Task: ctr})
	if err != nil {
		return err
	}
	return nil
}

func (c *clientV1) Update(ctx context.Context, id string, ctr *tasksv1.Task) error {
	tracer := otel.Tracer("client-v1")
	ctx, span := tracer.Start(ctx, "client.task.Update")
	defer span.End()

	uid, err := keys.ParseStr(id)
	if err != nil {
		return err
	}

	ctx = metadata.AppendToOutgoingContext(ctx, "voiyd_client_id", c.id)
	_, err = c.Client.Update(ctx, &tasksv1.UpdateRequest{Uid: uid.UUIDStr(), Name: uid.NameStr(), Task: ctr})
	if err != nil {
		return err
	}
	return nil
}

func (c *clientV1) Patch(ctx context.Context, id string, ctr *tasksv1.Task) error {
	tracer := otel.Tracer("client-v1")
	ctx, span := tracer.Start(ctx, "client.task.Patch")
	defer span.End()

	uid, err := keys.ParseStr(id)
	if err != nil {
		return err
	}

	ctx = metadata.AppendToOutgoingContext(ctx, "voiyd_client_id", c.id)
	_, err = c.Client.Patch(ctx, &tasksv1.PatchRequest{Uid: uid.UUIDStr(), Name: uid.NameStr(), Task: ctr})
	if err != nil {
		return err
	}
	return nil
}

func (c *clientV1) Get(ctx context.Context, id string) (*tasksv1.Task, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, "voiyd_client_id", c.id)

	tracer := otel.Tracer("client-v1")
	ctx, span := tracer.Start(ctx, "client.task.Get")
	defer span.End()

	uid, err := keys.ParseStr(id)
	if err != nil {
		return nil, err
	}

	res, err := c.Client.Get(ctx, &tasksv1.GetRequest{Uid: uid.UUIDStr(), Name: uid.NameStr()})
	if err != nil {
		return nil, err
	}
	return res.GetTask(), nil
}

func (c *clientV1) List(ctx context.Context, l ...labels.Label) ([]*tasksv1.Task, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, "voiyd_client_id", c.id)

	tracer := otel.Tracer("client-v1")
	ctx, span := tracer.Start(ctx, "client.task.List")
	defer span.End()

	mergedLabels := util.MergeLabels(l...)
	res, err := c.Client.List(ctx, &tasksv1.ListRequest{Selector: mergedLabels})
	if err != nil {
		return nil, err
	}
	return res.Tasks, nil
}

func (c *clientV1) Delete(ctx context.Context, id string) error {
	ctx = metadata.AppendToOutgoingContext(ctx, "voiyd_client_id", c.id)

	tracer := otel.Tracer("client-v1")
	ctx, span := tracer.Start(ctx, "client.task.Delete")
	defer span.End()

	uid, err := keys.ParseStr(id)
	if err != nil {
		return err
	}

	_, err = c.Client.Delete(ctx, &tasksv1.DeleteRequest{Uid: uid.UUIDStr(), Name: uid.NameStr()})
	if err != nil {
		return err
	}
	return nil
}

func (c *clientV1) Condition(ctx context.Context, taskID string, conditions ...*typesv1.Condition) error {
	if _, err := c.Client.Condition(ctx, &typesv1.ConditionRequest{ResourceVersion: task.Version, Conditions: conditions, TaskId: taskID}); err != nil {
		return err
	}
	return nil
}

func NewClientV1(opts ...CreateOption) ClientV1 {
	c := &clientV1{}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func NewClientV1WithConn(conn *grpc.ClientConn, clientID string, opts ...CreateOption) ClientV1 {
	c := &clientV1{
		Client: tasksv1.NewTaskServiceClient(conn),
		id:     clientID,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}
