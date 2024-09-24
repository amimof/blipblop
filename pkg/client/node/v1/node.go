package v1

import (
	"context"
	"net"
	"os"
	"runtime"

	"github.com/amimof/blipblop/api/services/nodes/v1"
	"github.com/amimof/blipblop/api/types/v1"
	"github.com/amimof/blipblop/services"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

type NodeV1Client struct {
	name        string
	nodeService nodes.NodeServiceClient
}

func getIpAddressesAsString() []string {
	var i []string
	inters, err := net.Interfaces()
	if err != nil {
		return i
	}
	for _, inter := range inters {
		addrs, err := inter.Addrs()
		if err != nil {
			return i
		}
		for _, addr := range addrs {
			a := addr.String()
			i = append(i, a)
		}
	}
	return i
}

func (c *NodeV1Client) NodeService() nodes.NodeServiceClient {
	return c.nodeService
}

func (c *NodeV1Client) DeleteNode(ctx context.Context, id string) error {
	_, err := c.nodeService.Delete(ctx, &nodes.DeleteNodeRequest{Id: id})
	if err != nil {
		return err
	}
	return nil
}

func (c *NodeV1Client) GetNode(ctx context.Context, id string) (*nodes.Node, error) {
	n, err := c.nodeService.Get(ctx, &nodes.GetNodeRequest{Id: id})
	if err != nil {
		return nil, err
	}
	return n.GetNode(), nil
}

func (c *NodeV1Client) ListNodes(ctx context.Context) ([]*nodes.Node, error) {
	n, err := c.nodeService.List(ctx, &nodes.ListNodeRequest{})
	if err != nil {
		return nil, err
	}
	return n.Nodes, nil
}

func (c *NodeV1Client) UpdateNode(ctx context.Context, node *nodes.Node) error {
	err := services.EnsureMeta(node)
	if err != nil {
		return err
	}
	// node.Updated = timestamppb.New(time.Now())
	// node.Revision = node.Revision + 1
	_, err = c.nodeService.Update(ctx, &nodes.UpdateNodeRequest{Node: node})
	if err != nil {
		return err
	}
	return nil
}

func (c *NodeV1Client) JoinNode(ctx context.Context, node *nodes.Node) error {
	// node.Created = timestamppb.New(time.Now())
	// node.Updated = timestamppb.New(time.Now())
	// node.Revision = 1
	err := services.EnsureMeta(node)
	if err != nil {
		return err
	}
	c.name = node.GetMeta().GetName()
	_, err = c.nodeService.Join(ctx, &nodes.JoinRequest{Node: node})
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

// NewNodeFromEnv creates a new node from the current environment with the name s
func NewNodeFromEnv(s string) *nodes.Node {
	arch := runtime.GOARCH
	oper := runtime.GOOS
	hostname, _ := os.Hostname()
	n := &nodes.Node{
		Meta: &types.Meta{
			Name: s,
		},
		Status: &nodes.Status{
			Ips:      getIpAddressesAsString(),
			Hostname: hostname,
			Arch:     arch,
			Os:       oper,
			Ready:    false,
		},
	}
	return n
}
