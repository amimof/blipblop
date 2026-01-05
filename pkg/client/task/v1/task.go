package v1

import (
	"context"

	"go.opentelemetry.io/otel"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"github.com/amimof/voiyd/pkg/labels"
	"github.com/amimof/voiyd/pkg/util"
	"github.com/amimof/voiyd/services/task"

	tasksv1 "github.com/amimof/voiyd/api/services/tasks/v1"
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
	Kill(context.Context, string) (*tasksv1.KillResponse, error)
	Stop(context.Context, string) (*tasksv1.KillResponse, error)
	Start(context.Context, string) (*tasksv1.StartResponse, error)
	Create(context.Context, *tasksv1.Task, ...CreateOption) error
	Update(context.Context, string, *tasksv1.Task) error
	Patch(context.Context, string, *tasksv1.Task) error
	Get(context.Context, string) (*tasksv1.Task, error)
	Delete(context.Context, string) error
	List(context.Context, ...labels.Label) ([]*tasksv1.Task, error)
}

type StatusClientV1 interface {
	Update(context.Context, string, *tasksv1.Status, ...string) error
}

type clientV1 struct {
	Client     tasksv1.TaskServiceClient
	emitLabels labels.Label
	id         string
}

type statusV1 struct {
	client tasksv1.TaskServiceClient
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

	req := &tasksv1.UpdateStatusRequest{
		Id:         id,
		UpdateMask: mask,
		Status:     status,
	}

	_, err := c.client.UpdateStatus(ctx, req)
	if err != nil {
		return err
	}

	return nil
}

func (c *clientV1) Kill(ctx context.Context, id string) (*tasksv1.KillResponse, error) {
	tracer := otel.Tracer("client-v1")
	ctx, span := tracer.Start(ctx, "client.task.Kill")
	defer span.End()

	ctx = metadata.AppendToOutgoingContext(ctx, "voiyd_client_id", c.id)
	resp, err := c.Client.Kill(ctx, &tasksv1.KillRequest{Id: id, ForceKill: true})
	if err != nil {
		return nil, err
	}
	return resp, err
}

func (c *clientV1) Stop(ctx context.Context, id string) (*tasksv1.KillResponse, error) {
	tracer := otel.Tracer("client-v1")
	ctx, span := tracer.Start(ctx, "client.task.Start")
	defer span.End()

	ctx = metadata.AppendToOutgoingContext(ctx, "voiyd_client_id", c.id)
	resp, err := c.Client.Kill(ctx, &tasksv1.KillRequest{Id: id, ForceKill: false})
	if err != nil {
		return nil, err
	}
	return resp, err
}

func (c *clientV1) Start(ctx context.Context, id string) (*tasksv1.StartResponse, error) {
	tracer := otel.Tracer("client-v1")
	ctx, span := tracer.Start(ctx, "client.task.Start")
	defer span.End()

	ctx = metadata.AppendToOutgoingContext(ctx, "voiyd_client_id", c.id)
	resp, err := c.Client.Start(ctx, &tasksv1.StartRequest{Id: id})
	if err != nil {
		return nil, err
	}

	return resp, err
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

	ctx = metadata.AppendToOutgoingContext(ctx, "voiyd_client_id", c.id)
	_, err := c.Client.Update(ctx, &tasksv1.UpdateRequest{Id: id, Task: ctr})
	if err != nil {
		return err
	}
	return nil
}

func (c *clientV1) Patch(ctx context.Context, id string, ctr *tasksv1.Task) error {
	tracer := otel.Tracer("client-v1")
	ctx, span := tracer.Start(ctx, "client.task.Patch")
	defer span.End()

	ctx = metadata.AppendToOutgoingContext(ctx, "voiyd_client_id", c.id)
	_, err := c.Client.Patch(ctx, &tasksv1.PatchRequest{Id: id, Task: ctr})
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

	res, err := c.Client.Get(ctx, &tasksv1.GetRequest{Id: id})
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
	_, err := c.Client.Delete(ctx, &tasksv1.DeleteRequest{Id: id})
	if err != nil {
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

func NewClientV1WithConn(conn *grpc.ClientConn, clientId string, opts ...CreateOption) ClientV1 {
	c := &clientV1{
		Client: tasksv1.NewTaskServiceClient(conn),
		id:     clientId,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}
