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

type CreateOption func(c *ClientV1) error

func WithEmitLabels(l labels.Label) CreateOption {
	return func(c *ClientV1) error {
		c.emitLabels = l
		return nil
	}
}

type ClientV1 struct {
	Client     containers.ContainerServiceClient
	emitLabels labels.Label
	id         string
}

func (c *ClientV1) SetNode(ctx context.Context, id, node string) error {
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

func (c *ClientV1) SetTaskStatus(ctx context.Context, id string, phase string) error {
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

// func (c *ClientV1) SetTaskReason(ctx context.Context, id string, reason string) error {
// 	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_client_id", c.id)
// 	n := &containers.UpdateContainerRequest{
// 		Id: id,
// 		Container: &containers.Container{
// 			Status: &containers.Status{
// 				Reason: reason,
// 			},
// 		},
// 		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"status.reason"}},
// 	}
// 	_, err := c.Client.Update(ctx, n)
// 	if err != nil {
// 		return err
// 	}
// 	return nil
// }

func (c *ClientV1) SetStatus(ctx context.Context, id string, status *containers.Status) error {
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

func (c *ClientV1) Kill(ctx context.Context, id string) (*containers.KillContainerResponse, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_client_id", c.id)
	resp, err := c.Client.Kill(ctx, &containers.KillContainerRequest{Id: id, ForceKill: true})
	if err != nil {
		return nil, err
	}
	return resp, err
}

func (c *ClientV1) Stop(ctx context.Context, id string) (*containers.KillContainerResponse, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_client_id", c.id)
	resp, err := c.Client.Kill(ctx, &containers.KillContainerRequest{Id: id, ForceKill: false})
	if err != nil {
		return nil, err
	}
	return resp, err
}

func (c *ClientV1) Start(ctx context.Context, id string) (*containers.StartContainerResponse, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_client_id", c.id)
	resp, err := c.Client.Start(ctx, &containers.StartContainerRequest{Id: id})
	if err != nil {
		return nil, err
	}

	return resp, err
}

func (c *ClientV1) Create(ctx context.Context, ctr *containers.Container, opts ...CreateOption) error {
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

func (c *ClientV1) Update(ctx context.Context, id string, ctr *containers.Container) error {
	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_client_id", c.id)
	_, err := c.Client.Update(ctx, &containers.UpdateContainerRequest{Id: id, Container: ctr})
	if err != nil {
		return err
	}
	return nil
}

func (c *ClientV1) Get(ctx context.Context, id string) (*containers.Container, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_client_id", c.id)
	res, err := c.Client.Get(ctx, &containers.GetContainerRequest{Id: id})
	if err != nil {
		return nil, err
	}
	return res.GetContainer(), nil
}

func (c *ClientV1) List(ctx context.Context, l ...labels.Label) ([]*containers.Container, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_client_id", c.id)

	mergedLabels := util.MergeLabels(l...)
	res, err := c.Client.List(ctx, &containers.ListContainerRequest{Selector: mergedLabels})
	if err != nil {
		return nil, err
	}
	return res.Containers, nil
}

func (c *ClientV1) Delete(ctx context.Context, id string) error {
	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_client_id", c.id)
	_, err := c.Client.Delete(ctx, &containers.DeleteContainerRequest{Id: id})
	if err != nil {
		return err
	}
	return nil
}

func NewClientV1(conn *grpc.ClientConn, clientId string) *ClientV1 {
	return &ClientV1{
		Client: containers.NewContainerServiceClient(conn),
		id:     clientId,
	}
}
