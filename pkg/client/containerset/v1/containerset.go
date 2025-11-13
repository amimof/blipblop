package v1

import (
	"context"
	"fmt"
	"reflect"

	containersetsv1 "github.com/amimof/blipblop/api/services/containersets/v1"
	"github.com/amimof/blipblop/pkg/labels"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

var (
	ContainerHealthHealthy   = "healthy"
	ContainerHealthUnhealthy = "unhealthy"
)

type ClientV1 struct {
	containerService containersetsv1.ContainerSetServiceClient
	id               string
}

type Status = status.Status

type Response[T any] struct {
	Status Status
	Raw    proto.Message
}

func (g *Response[T]) Object() (T, error) {
	// Attempt to cast the value inside GenericContainer to type T
	v, ok := g.Raw.(T)
	if !ok {
		var zero T
		return zero, fmt.Errorf("failed to convert %v (type %s) to type %s", g.Raw, reflect.TypeOf(g.Raw), reflect.TypeOf(zero))
	}
	return v, nil
}

func (c *ClientV1) Create(ctx context.Context, ctr *containersetsv1.ContainerSet) error {
	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_client_id", c.id)
	_, err := c.containerService.Create(ctx, &containersetsv1.CreateRequest{ContainerSet: ctr})
	if err != nil {
		return err
	}
	return nil
}

func (c *ClientV1) Get(ctx context.Context, id string) (*containersetsv1.ContainerSet, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_client_id", c.id)
	res, err := c.containerService.Get(ctx, &containersetsv1.GetRequest{Id: id})
	if err != nil {
		return nil, err
	}
	return res.GetContainerSet(), nil
}

func (c *ClientV1) List(ctx context.Context) ([]*containersetsv1.ContainerSet, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_client_id", c.id)
	res, err := c.containerService.List(ctx, &containersetsv1.ListRequest{Selector: labels.New()})
	if err != nil {
		return nil, err
	}
	return res.ContainerSets, nil
}

func (c *ClientV1) Delete(ctx context.Context, id string) error {
	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_client_id", c.id)
	_, err := c.containerService.Delete(ctx, &containersetsv1.DeleteRequest{Id: id})
	if err != nil {
		return err
	}
	return nil
}

func NewClientV1(conn *grpc.ClientConn, clientId string) *ClientV1 {
	return &ClientV1{
		containerService: containersetsv1.NewContainerSetServiceClient(conn),
		id:               clientId,
	}
}
