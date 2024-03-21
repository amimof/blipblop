package v1

import (
	"context"

	"github.com/amimof/blipblop/api/services/containers/v1"
	"github.com/amimof/blipblop/pkg/labels"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

type ContainerV1Client struct {
	containerService containers.ContainerServiceClient
}

func (c *ContainerV1Client) NodeService() containers.ContainerServiceClient {
	return c.containerService
}

func (c *ContainerV1Client) SetContainerNode(ctx context.Context, id, node string) error {
	n := &containers.UpdateContainerRequest{
		Container: &containers.Container{
			Name: id,
			Status: &containers.Status{
				Node: node,
			},
		},
	}
	fm, err := fieldmaskpb.New(n.Container, "status.node")
	if err != nil {
		return err
	}
	fm.Normalize()
	n.UpdateMask = fm
	if fm.IsValid(n.Container) {
		_, err = c.containerService.Update(ctx, n)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *ContainerV1Client) SetContainerState(ctx context.Context, id, state string) error {
	n := &containers.UpdateContainerRequest{
		Container: &containers.Container{
			Name: id,
			Status: &containers.Status{
				State: state,
			},
		},
	}
	fm, err := fieldmaskpb.New(n.Container, "status.state")
	if err != nil {
		return err
	}
	fm.Normalize()
	n.UpdateMask = fm
	if fm.IsValid(n.Container) {
		_, err = c.containerService.Update(ctx, n)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *ContainerV1Client) RunContainer(ctx context.Context, id string) error {
	_, err := c.containerService.Start(ctx, &containers.StartContainerRequest{Id: id})
	if err != nil {
		return err
	}
	return nil
}

func (c *ContainerV1Client) CreateContainer(ctx context.Context, ctr *containers.Container) error {
	_, err := c.containerService.Create(ctx, &containers.CreateContainerRequest{Container: ctr})
	if err != nil {
		return err
	}
	return nil
}

func (c *ContainerV1Client) GetContainer(ctx context.Context, id string) (*containers.Container, error) {
	res, err := c.containerService.Get(ctx, &containers.GetContainerRequest{Id: id})
	if err != nil {
		return nil, err
	}
	return res.Container, nil
}

func (c *ContainerV1Client) ListContainers(ctx context.Context) ([]*containers.Container, error) {
	res, err := c.containerService.List(ctx, &containers.ListContainerRequest{Selector: labels.New()})
	if err != nil {
		return nil, err
	}
	return res.Containers, nil
}

func (c *ContainerV1Client) DeleteContainer(ctx context.Context, id string) error {
	_, err := c.containerService.Delete(ctx, &containers.DeleteContainerRequest{Id: id})
	if err != nil {
		return err
	}
	return nil
}

func NewContainerV1Client(conn *grpc.ClientConn) *ContainerV1Client {
	return &ContainerV1Client{
		containerService: containers.NewContainerServiceClient(conn),
	}
}
