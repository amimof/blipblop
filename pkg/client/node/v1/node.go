package v1

import (
	"context"

	"github.com/amimof/blipblop/api/services/nodes/v1"
	"github.com/amimof/blipblop/services"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

type ClientV1 struct {
	nodeService nodes.NodeServiceClient
	id          string
}

func (c *ClientV1) NodeService() nodes.NodeServiceClient {
	return c.nodeService
}

func (c *ClientV1) Delete(ctx context.Context, id string) error {
	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_client_id", c.id)
	_, err := c.nodeService.Delete(ctx, &nodes.DeleteNodeRequest{Id: id})
	if err != nil {
		return err
	}
	return nil
}

func (c *ClientV1) Get(ctx context.Context, id string) (*nodes.Node, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_client_id", c.id)
	n, err := c.nodeService.Get(ctx, &nodes.GetNodeRequest{Id: id})
	if err != nil {
		return nil, err
	}
	return n.GetNode(), nil
}

func (c *ClientV1) List(ctx context.Context) ([]*nodes.Node, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_client_id", c.id)
	n, err := c.nodeService.List(ctx, &nodes.ListNodeRequest{})
	if err != nil {
		return nil, err
	}
	return n.Nodes, nil
}

func (c *ClientV1) Update(ctx context.Context, node *nodes.Node) error {
	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_client_id", c.id)

	m, err := services.EnsureMeta(node)
	if err != nil {
		return err
	}
	node.Meta = m

	// node.Updated = timestamppb.New(time.Now())
	// node.Revision = node.Revision + 1
	_, err = c.nodeService.Update(ctx, &nodes.UpdateNodeRequest{Id: node.GetMeta().GetName(), Node: node})
	if err != nil {
		return err
	}
	return nil
}

func (c *ClientV1) Join(ctx context.Context, node *nodes.Node) error {
	// ctx = context.WithValue(ctx, "blipblop/client_id", c.id)
	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_client_id", c.id)
	// node.Created = timestamppb.New(time.Now())
	// node.Updated = timestamppb.New(time.Now())
	// node.Revision = 1
	m, err := services.EnsureMeta(node)
	if err != nil {
		return err
	}
	node.Meta = m
	// c.name = node.GetMeta().GetName()
	_, err = c.nodeService.Join(ctx, &nodes.JoinRequest{Node: node})
	if err != nil {
		return err
	}
	return nil
}

func (c *ClientV1) Forget(ctx context.Context, n string) error {
	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_client_id", c.id)
	req := &nodes.ForgetRequest{
		Id: n,
	}
	_, err := c.nodeService.Forget(ctx, req)
	if err != nil {
		return err
	}
	return nil
}

func (c *ClientV1) SetState(ctx context.Context, nodeName string, state connectivity.State) error {
	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_client_id", c.id)
	n := &nodes.UpdateNodeRequest{
		Id: nodeName,
		Node: &nodes.Node{
			Status: &nodes.Status{
				State: state.String(),
			},
		},
	}
	fm, err := fieldmaskpb.New(n.Node, "status.state")
	if err != nil {
		return err
	}
	fm.Normalize()
	n.UpdateMask = fm
	if fm.IsValid(n.Node) {
		_, err = c.nodeService.Update(ctx, n)
		if err != nil {
			return err
		}
	}
	return nil
}

func NewClientV1(conn *grpc.ClientConn, clientId string) *ClientV1 {
	return &ClientV1{
		nodeService: nodes.NewNodeServiceClient(conn),
		id:          clientId,
	}
}
