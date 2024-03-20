package v1

import (
	"context"
	"time"

	"github.com/amimof/blipblop/api/services/nodes/v1"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type NodeV1Client struct {
	name        string
	nodeService nodes.NodeServiceClient
}

func (c *NodeV1Client) NodeService() nodes.NodeServiceClient {
	return c.nodeService
}

func (c *NodeV1Client) UpdateNode(ctx context.Context, node *nodes.Node) error {
	node.Updated = timestamppb.New(time.Now())
	node.Revision = node.Revision + 1
	_, err := c.nodeService.Update(ctx, &nodes.UpdateNodeRequest{Node: node})
	if err != nil {
		return err
	}
	return nil
}

func (c *NodeV1Client) JoinNode(ctx context.Context, node *nodes.Node) error {
	c.name = node.Name
	node.Created = timestamppb.New(time.Now())
	node.Updated = timestamppb.New(time.Now())
	node.Revision = 1
	_, err := c.nodeService.Join(ctx, &nodes.JoinRequest{Node: node})
	if err != nil {
		return err
	}
	return nil
}

func (c *NodeV1Client) ForgetNode(ctx context.Context, n string) error {
	req := &nodes.ForgetRequest{
		Id: n,
	}
	_, err := c.nodeService.Forget(ctx, req)
	if err != nil {
		return err
	}
	return nil
}

func (c *NodeV1Client) SetNodeReady(ctx context.Context, ready bool) error {
	n := &nodes.UpdateNodeRequest{
		Node: &nodes.Node{
			Name: c.name,
			Status: &nodes.Status{
				Ready: ready,
			},
		},
	}
	fm, err := fieldmaskpb.New(n.Node, "status.ready")
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

func NewNodeV1Client(conn *grpc.ClientConn) *NodeV1Client {
	return &NodeV1Client{
		nodeService: nodes.NewNodeServiceClient(conn),
	}
}
