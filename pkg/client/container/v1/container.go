package v1

import (
	"context"

	"github.com/amimof/blipblop/api/services/containers/v1"
	"github.com/amimof/blipblop/pkg/labels"
	"github.com/amimof/blipblop/pkg/util"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

var (
	ContainerHealthHealthy   = "healthy"
	ContainerHealthUnhealthy = "unhealthy"
)

type CreateOption func(c *clientV1) error

func WithEmitLabels(l labels.Label) CreateOption {
	return func(c *clientV1) error {
		c.emitLabels = l
		return nil
	}
}

func WithClient(client containers.ContainerServiceClient) CreateOption {
	return func(c *clientV1) error {
		c.Client = client
		return nil
	}
}

type ClientV1 interface {
	Status(context.Context, string, *containers.Status) error

	// DEPRECATED: Use Status() instead
	SetNode(context.Context, string, string) error
	// DEPRECATED: Use Status() instead
	SetTaskStatus(context.Context, string, string) error
	// DEPRECATED: Use Status() instead
	// SetStatus(context.Context, string, *containers.Status) error

	Kill(context.Context, string) (*containers.KillContainerResponse, error)
	Stop(context.Context, string) (*containers.KillContainerResponse, error)
	Start(context.Context, string) (*containers.StartContainerResponse, error)
	Create(context.Context, *containers.Container, ...CreateOption) error
	Update(context.Context, string, *containers.Container) error
	Get(context.Context, string) (*containers.Container, error)
	Delete(context.Context, string) error
	List(context.Context, ...labels.Label) ([]*containers.Container, error)
}

type clientV1 struct {
	Client     containers.ContainerServiceClient
	emitLabels labels.Label
	id         string
}

func (c *clientV1) Status(ctx context.Context, id string, status *containers.Status) error {
	ctr, err := c.Get(ctx, id)
	if err != nil {
		return err
	}
	ctr.Status = status
	err = c.Update(ctx, id, ctr)
	if err != nil {
		return err
	}
	return err
}

func (c *clientV1) SetNode(ctx context.Context, id, node string) error {
	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_client_id", c.id)
	n := &containers.UpdateContainerRequest{
		Id: id,
		Container: &containers.Container{
			Status: &containers.Status{
				Node: node,
			},
		},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"status.node"}},
	}
	_, err := c.Client.Update(ctx, n)
	if err != nil {
		return err
	}
	return nil
}

func (c *clientV1) SetTaskStatus(ctx context.Context, id string, phase string) error {
	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_client_id", c.id)
	n := &containers.UpdateContainerRequest{
		Id: id,
		Container: &containers.Container{
			Status: &containers.Status{
				Phase: phase,
			},
		},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"status.phase"}},
	}
	_, err := c.Client.Update(ctx, n)
	if err != nil {
		return err
	}
	return nil
}

func (c *clientV1) SetStatus(ctx context.Context, id string, status *containers.Status) error {
	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_client_id", c.id)
	n := &containers.UpdateContainerRequest{
		Id: id,
		Container: &containers.Container{
			Status: status,
		},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"status"}},
	}
	_, err := c.Client.Update(ctx, n)
	if err != nil {
		return err
	}
	return nil
}

func (c *clientV1) Kill(ctx context.Context, id string) (*containers.KillContainerResponse, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_client_id", c.id)
	resp, err := c.Client.Kill(ctx, &containers.KillContainerRequest{Id: id, ForceKill: true})
	if err != nil {
		return nil, err
	}
	return resp, err
}

func (c *clientV1) Stop(ctx context.Context, id string) (*containers.KillContainerResponse, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_client_id", c.id)
	resp, err := c.Client.Kill(ctx, &containers.KillContainerRequest{Id: id, ForceKill: false})
	if err != nil {
		return nil, err
	}
	return resp, err
}

func (c *clientV1) Start(ctx context.Context, id string) (*containers.StartContainerResponse, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_client_id", c.id)
	resp, err := c.Client.Start(ctx, &containers.StartContainerRequest{Id: id})
	if err != nil {
		return nil, err
	}

	return resp, err
}

func (c *clientV1) Create(ctx context.Context, ctr *containers.Container, opts ...CreateOption) error {
	for _, opt := range opts {
		err := opt(c)
		if err != nil {
			return err
		}
	}
	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_client_id", c.id)
	_, err := c.Client.Create(ctx, &containers.CreateContainerRequest{Container: ctr})
	if err != nil {
		return err
	}
	return nil
}

func (c *clientV1) Update(ctx context.Context, id string, ctr *containers.Container) error {
	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_client_id", c.id)
	_, err := c.Client.Update(ctx, &containers.UpdateContainerRequest{Id: id, Container: ctr})
	if err != nil {
		return err
	}
	return nil
}

func (c *clientV1) Get(ctx context.Context, id string) (*containers.Container, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_client_id", c.id)
	res, err := c.Client.Get(ctx, &containers.GetContainerRequest{Id: id})
	if err != nil {
		return nil, err
	}
	return res.GetContainer(), nil
}

func (c *clientV1) List(ctx context.Context, l ...labels.Label) ([]*containers.Container, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_client_id", c.id)

	mergedLabels := util.MergeLabels(l...)
	res, err := c.Client.List(ctx, &containers.ListContainerRequest{Selector: mergedLabels})
	if err != nil {
		return nil, err
	}
	return res.Containers, nil
}

func (c *clientV1) Delete(ctx context.Context, id string) error {
	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_client_id", c.id)
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
