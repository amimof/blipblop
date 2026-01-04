package v1

import (
	"context"

	"go.opentelemetry.io/otel"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	containersv1 "github.com/amimof/voiyd/api/services/containers/v1"
	"github.com/amimof/voiyd/pkg/labels"
	"github.com/amimof/voiyd/pkg/util"
	"github.com/amimof/voiyd/services/container"
)

const (
	ContainerHealthHealthy   = "healthy"
	ContainerHealthUnhealthy = "unhealthy"
)

type CreateOption func(c *clientV1)

func WithEmitLabels(l labels.Label) CreateOption {
	return func(c *clientV1) {
		c.emitLabels = l
	}
}

func WithClient(client containersv1.ContainerServiceClient) CreateOption {
	return func(c *clientV1) {
		c.Client = client
	}
}

type ClientV1 interface {
	Status() StatusClientV1
	Kill(context.Context, string) (*containersv1.KillResponse, error)
	Stop(context.Context, string) (*containersv1.KillResponse, error)
	Start(context.Context, string) (*containersv1.StartResponse, error)
	Create(context.Context, *containersv1.Container, ...CreateOption) error
	Update(context.Context, string, *containersv1.Container) error
	Patch(context.Context, string, *containersv1.Container) error
	Get(context.Context, string) (*containersv1.Container, error)
	Delete(context.Context, string) error
	List(context.Context, ...labels.Label) ([]*containersv1.Container, error)
}

type StatusClientV1 interface {
	Update(context.Context, string, *containersv1.Status, ...string) error
}

type clientV1 struct {
	Client     containersv1.ContainerServiceClient
	emitLabels labels.Label
	id         string
}

type statusV1 struct {
	client containersv1.ContainerServiceClient
}

func (c *clientV1) Status() StatusClientV1 {
	return &statusV1{
		client: c.Client,
	}
}

func (c *statusV1) Update(ctx context.Context, id string, status *containersv1.Status, path ...string) error {
	// Construct field mask
	mask := &fieldmaskpb.FieldMask{
		Paths: path,
	}

	req := &containersv1.UpdateStatusRequest{
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

func (c *clientV1) Kill(ctx context.Context, id string) (*containersv1.KillResponse, error) {
	tracer := otel.Tracer("client-v1")
	ctx, span := tracer.Start(ctx, "client.container.Kill")
	defer span.End()

	ctx = metadata.AppendToOutgoingContext(ctx, "voiyd_client_id", c.id)
	resp, err := c.Client.Kill(ctx, &containersv1.KillRequest{Id: id, ForceKill: true})
	if err != nil {
		return nil, err
	}
	return resp, err
}

func (c *clientV1) Stop(ctx context.Context, id string) (*containersv1.KillResponse, error) {
	tracer := otel.Tracer("client-v1")
	ctx, span := tracer.Start(ctx, "client.container.Start")
	defer span.End()

	ctx = metadata.AppendToOutgoingContext(ctx, "voiyd_client_id", c.id)
	resp, err := c.Client.Kill(ctx, &containersv1.KillRequest{Id: id, ForceKill: false})
	if err != nil {
		return nil, err
	}
	return resp, err
}

func (c *clientV1) Start(ctx context.Context, id string) (*containersv1.StartResponse, error) {
	tracer := otel.Tracer("client-v1")
	ctx, span := tracer.Start(ctx, "client.container.Start")
	defer span.End()

	ctx = metadata.AppendToOutgoingContext(ctx, "voiyd_client_id", c.id)
	resp, err := c.Client.Start(ctx, &containersv1.StartRequest{Id: id})
	if err != nil {
		return nil, err
	}

	return resp, err
}

func (c *clientV1) Create(ctx context.Context, ctr *containersv1.Container, opts ...CreateOption) error {
	tracer := otel.Tracer("client-v1")
	ctx, span := tracer.Start(ctx, "client.container.Update")
	defer span.End()

	if ctr.Version == "" {
		ctr.Version = container.Version
	}

	for _, opt := range opts {
		opt(c)
	}

	ctx = metadata.AppendToOutgoingContext(ctx, "voiyd_client_id", c.id)
	_, err := c.Client.Create(ctx, &containersv1.CreateRequest{Container: ctr})
	if err != nil {
		return err
	}
	return nil
}

func (c *clientV1) Update(ctx context.Context, id string, ctr *containersv1.Container) error {
	tracer := otel.Tracer("client-v1")
	ctx, span := tracer.Start(ctx, "client.container.Update")
	defer span.End()

	ctx = metadata.AppendToOutgoingContext(ctx, "voiyd_client_id", c.id)
	_, err := c.Client.Update(ctx, &containersv1.UpdateRequest{Id: id, Container: ctr})
	if err != nil {
		return err
	}
	return nil
}

func (c *clientV1) Patch(ctx context.Context, id string, ctr *containersv1.Container) error {
	tracer := otel.Tracer("client-v1")
	ctx, span := tracer.Start(ctx, "client.container.Patch")
	defer span.End()

	ctx = metadata.AppendToOutgoingContext(ctx, "voiyd_client_id", c.id)
	_, err := c.Client.Patch(ctx, &containersv1.PatchRequest{Id: id, Container: ctr})
	if err != nil {
		return err
	}
	return nil
}

func (c *clientV1) Get(ctx context.Context, id string) (*containersv1.Container, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, "voiyd_client_id", c.id)

	tracer := otel.Tracer("client-v1")
	ctx, span := tracer.Start(ctx, "client.container.Get")
	defer span.End()

	res, err := c.Client.Get(ctx, &containersv1.GetRequest{Id: id})
	if err != nil {
		return nil, err
	}
	return res.GetContainer(), nil
}

func (c *clientV1) List(ctx context.Context, l ...labels.Label) ([]*containersv1.Container, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, "voiyd_client_id", c.id)

	tracer := otel.Tracer("client-v1")
	ctx, span := tracer.Start(ctx, "client.container.List")
	defer span.End()

	mergedLabels := util.MergeLabels(l...)
	res, err := c.Client.List(ctx, &containersv1.ListRequest{Selector: mergedLabels})
	if err != nil {
		return nil, err
	}
	return res.Containers, nil
}

func (c *clientV1) Delete(ctx context.Context, id string) error {
	ctx = metadata.AppendToOutgoingContext(ctx, "voiyd_client_id", c.id)

	tracer := otel.Tracer("client-v1")
	ctx, span := tracer.Start(ctx, "client.container.Delete")

	defer span.End()
	_, err := c.Client.Delete(ctx, &containersv1.DeleteRequest{Id: id})
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
		Client: containersv1.NewContainerServiceClient(conn),
		id:     clientId,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}
