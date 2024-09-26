package v1

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/amimof/blipblop/api/services/containers/v1"
	"github.com/amimof/blipblop/pkg/labels"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	ContainerHealthHealthy   = "healthy"
	ContainerHealthUnhealthy = "unhealthy"
)

type ClientV1 struct {
	containerService containers.ContainerServiceClient
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

func (c *ClientV1) SetNode(ctx context.Context, id, node string) error {
	n := &containers.UpdateContainerRequest{
		Id: id,
		Container: &containers.Container{
			Status: &containers.Status{
				Node: node,
			},
		},
	}
	fm, err := fieldmaskpb.New(n.Container, "status.node")
	if err != nil {
		return handleError(err)
	}
	fm.Normalize()
	n.UpdateMask = fm
	if fm.IsValid(n.Container) {
		_, err = c.containerService.Update(ctx, n)
		if err != nil {
			return handleError(err)
		}
	}
	return nil
}

func (c *ClientV1) SetHealth(ctx context.Context, id, health string) error {
	n := &containers.UpdateContainerRequest{
		Id: id,
		Container: &containers.Container{
			Status: &containers.Status{
				Health: health,
			},
		},
	}
	fm, err := fieldmaskpb.New(n.Container, "status.health")
	if err != nil {
		return handleError(err)
	}
	fm.Normalize()
	n.UpdateMask = fm
	if fm.IsValid(n.Container) {
		_, err = c.containerService.Update(ctx, n)
		if err != nil {
			return handleError(err)
		}
	}
	return nil
}

func (c *ClientV1) SetStatus(ctx context.Context, id string, status *containers.Status) error {
	n := &containers.UpdateContainerRequest{
		Id: id,
		Container: &containers.Container{
			Status: status,
		},
	}
	fm, err := fieldmaskpb.New(n.Container, "status")
	if err != nil {
		return handleError(err)
	}
	fm.Normalize()
	n.UpdateMask = fm
	if fm.IsValid(n.Container) {
		_, err = c.containerService.Update(ctx, n)
		if err != nil {
			return handleError(err)
		}
	}
	return nil
}

func (c *ClientV1) AddEvent(ctx context.Context, id string, evt *containers.Event) error {
	ctr, err := c.Get(ctx, id)
	if err != nil {
		return handleError(err)
	}
	evt.Created = timestamppb.New(time.Now())
	events := ctr.GetEvents()
	events = append(events, evt)
	req := &containers.UpdateContainerRequest{
		Id: id,
		Container: &containers.Container{
			Events: events,
		},
	}
	fm, err := fieldmaskpb.New(req.Container, "events")
	if err != nil {
		return handleError(err)
	}
	fm.Normalize()
	req.UpdateMask = fm
	if fm.IsValid(req.Container) {
		_, err = c.containerService.Update(ctx, req)
		if err != nil {
			return handleError(err)
		}
	}
	return nil
}

func (c *ClientV1) Kill(ctx context.Context, id string) (*containers.KillContainerResponse, error) {
	resp, err := c.containerService.Kill(ctx, &containers.KillContainerRequest{Id: id})
	if err != nil {
		return nil, handleError(err)
	}
	return resp, err
}

func (c *ClientV1) Start(ctx context.Context, id string) (*containers.StartContainerResponse, error) {
	resp, err := c.containerService.Start(ctx, &containers.StartContainerRequest{Id: id})
	if err != nil {
		return nil, handleError(err)
	}

	return resp, err
}

func (c *ClientV1) Create(ctx context.Context, ctr *containers.Container) error {
	_, err := c.containerService.Create(ctx, &containers.CreateContainerRequest{Container: ctr})
	if err != nil {
		return handleError(err)
	}
	return nil
}

func (c *ClientV1) Get(ctx context.Context, id string) (*containers.Container, error) {
	res, err := c.containerService.Get(ctx, &containers.GetContainerRequest{Id: id})
	if err != nil {
		return nil, handleError(err)
	}
	return res.GetContainer(), nil
}

// Wrapper that decorates the error with grpc status error
func handleError(err error) error {
	st, ok := status.FromError(err)
	if ok {
		return fmt.Errorf("gRPC error: %s - %s", st.Code(), st.Message())
	}
	return fmt.Errorf("unknown error: %v", err)
}

func (c *ClientV1) List(ctx context.Context) ([]*containers.Container, error) {
	res, err := c.containerService.List(ctx, &containers.ListContainerRequest{Selector: labels.New()})
	if err != nil {
		return nil, handleError(err)
	}
	return res.Containers, nil
}

func (c *ClientV1) Delete(ctx context.Context, id string) error {
	_, err := c.containerService.Delete(ctx, &containers.DeleteContainerRequest{Id: id})
	if err != nil {
		return handleError(err)
	}
	return nil
}

func NewClientV1(conn *grpc.ClientConn) *ClientV1 {
	return &ClientV1{
		containerService: containers.NewContainerServiceClient(conn),
	}
}
