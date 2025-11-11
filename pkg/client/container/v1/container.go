package v1

import (
	"context"

	"github.com/amimof/blipblop/api/services/containers/v1"
	"github.com/amimof/blipblop/pkg/labels"
	"github.com/amimof/blipblop/pkg/util"
	"go.opentelemetry.io/otel"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

var (
	ContainerHealthHealthy   = "healthy"
	ContainerHealthUnhealthy = "unhealthy"
)

type CreateOption func(c *clientV1)

func WithEmitLabels(l labels.Label) CreateOption {
	return func(c *clientV1) {
		c.emitLabels = l
	}
}

func WithClient(client containers.ContainerServiceClient) CreateOption {
	return func(c *clientV1) {
		c.Client = client
	}
}

type ClientV1 interface {
	Status() StatusClientV1
	Kill(context.Context, string) (*containers.KillContainerResponse, error)
	Stop(context.Context, string) (*containers.KillContainerResponse, error)
	Start(context.Context, string) (*containers.StartContainerResponse, error)
	Create(context.Context, *containers.Container, ...CreateOption) error
	Update(context.Context, string, *containers.Container) error
	Patch(context.Context, string, *containers.Container) error
	Get(context.Context, string) (*containers.Container, error)
	Delete(context.Context, string) error
	List(context.Context, ...labels.Label) ([]*containers.Container, error)
}

type StatusClientV1 interface {
	Update(context.Context, string, *containers.Status, ...string) error
}

type clientV1 struct {
	Client     containers.ContainerServiceClient
	emitLabels labels.Label
	id         string
}

type statusV1 struct {
	client containers.ContainerServiceClient
}

func (s *clientV1) Status() StatusClientV1 {
	return &statusV1{
		client: s.Client,
	}
}

func (s *statusV1) Update(ctx context.Context, id string, status *containers.Status, path ...string) error {
	// Construct field mask
	mask := &fieldmaskpb.FieldMask{
		Paths: path,
	}

	req := &containers.UpdateStatusRequest{
		Id:         id,
		UpdateMask: mask,
		Status:     status,
	}

	_, err := s.client.UpdateStatus(ctx, req)
	if err != nil {
		return err
	}

	return nil
}

func (c *clientV1) Kill(ctx context.Context, id string) (*containers.KillContainerResponse, error) {
	tracer := otel.Tracer("client-v1")
	ctx, span := tracer.Start(ctx, "client.container.Kill")
	defer span.End()

	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_client_id", c.id)
	resp, err := c.Client.Kill(ctx, &containers.KillContainerRequest{Id: id, ForceKill: true})
	if err != nil {
		return nil, err
	}
	return resp, err
}

func (c *clientV1) Stop(ctx context.Context, id string) (*containers.KillContainerResponse, error) {
	tracer := otel.Tracer("client-v1")
	ctx, span := tracer.Start(ctx, "client.container.Start")
	defer span.End()

	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_client_id", c.id)
	resp, err := c.Client.Kill(ctx, &containers.KillContainerRequest{Id: id, ForceKill: false})
	if err != nil {
		return nil, err
	}
	return resp, err
}

func (c *clientV1) Start(ctx context.Context, id string) (*containers.StartContainerResponse, error) {
	tracer := otel.Tracer("client-v1")
	ctx, span := tracer.Start(ctx, "client.container.Start")
	defer span.End()

	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_client_id", c.id)
	resp, err := c.Client.Start(ctx, &containers.StartContainerRequest{Id: id})
	if err != nil {
		return nil, err
	}

	return resp, err
}

func (c *clientV1) Create(ctx context.Context, ctr *containers.Container, opts ...CreateOption) error {
	tracer := otel.Tracer("client-v1")
	ctx, span := tracer.Start(ctx, "client.container.Update")
	defer span.End()

	for _, opt := range opts {
		opt(c)
	}
	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_client_id", c.id)
	_, err := c.Client.Create(ctx, &containers.CreateContainerRequest{Container: ctr})
	if err != nil {
		return err
	}
	return nil
}

func (c *clientV1) Update(ctx context.Context, id string, ctr *containers.Container) error {
	tracer := otel.Tracer("client-v1")
	ctx, span := tracer.Start(ctx, "client.container.Update")
	defer span.End()

	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_client_id", c.id)
	_, err := c.Client.Update(ctx, &containers.UpdateContainerRequest{Id: id, Container: ctr})
	if err != nil {
		return err
	}
	return nil
}

func (c *clientV1) Patch(ctx context.Context, id string, ctr *containers.Container) error {
	tracer := otel.Tracer("client-v1")
	ctx, span := tracer.Start(ctx, "client.container.Patch")
	defer span.End()

	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_client_id", c.id)
	_, err := c.Client.Patch(ctx, &containers.UpdateContainerRequest{Id: id, Container: ctr})
	if err != nil {
		return err
	}
	return nil
}

func (c *clientV1) Get(ctx context.Context, id string) (*containers.Container, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_client_id", c.id)

	tracer := otel.Tracer("client-v1")
	ctx, span := tracer.Start(ctx, "client.container.Get")
	defer span.End()

	res, err := c.Client.Get(ctx, &containers.GetContainerRequest{Id: id})
	if err != nil {
		return nil, err
	}
	return res.GetContainer(), nil
}

func (c *clientV1) List(ctx context.Context, l ...labels.Label) ([]*containers.Container, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_client_id", c.id)

	tracer := otel.Tracer("client-v1")
	ctx, span := tracer.Start(ctx, "client.container.List")
	defer span.End()

	mergedLabels := util.MergeLabels(l...)
	res, err := c.Client.List(ctx, &containers.ListContainerRequest{Selector: mergedLabels})
	if err != nil {
		return nil, err
	}
	return res.Containers, nil
}

func (c *clientV1) Delete(ctx context.Context, id string) error {
	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_client_id", c.id)

	tracer := otel.Tracer("client-v1")
	ctx, span := tracer.Start(ctx, "client.container.Delete")

	defer span.End()
	_, err := c.Client.Delete(ctx, &containers.DeleteContainerRequest{Id: id})
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
		Client: containers.NewContainerServiceClient(conn),
		id:     clientId,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}
