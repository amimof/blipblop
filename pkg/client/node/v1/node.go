package v1

import (
	"context"
	"fmt"
	"net"
	"os"
	"runtime"
	"time"

	"github.com/amimof/blipblop/api/services/nodes/v1"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type NodeV1Client struct {
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

func (c *NodeV1Client) ListNodes(ctx context.Context) ([]*nodes.Node, error) {
	n, err := c.nodeService.List(ctx, &nodes.ListNodeRequest{})
	if err != nil {
		return nil, err
	}
	return n.Nodes, nil
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

func (c *NodeV1Client) SetNodeReady(ctx context.Context, n string, ready bool) error {
	node := &nodes.UpdateNodeRequest{
		Node: &nodes.Node{
			Name: n,
			Status: &nodes.Status{
				Ready: ready,
			},
		},
	}
	fmt.Println(node)
	fm, err := fieldmaskpb.New(node.Node, "status.ready")
	if err != nil {
		return err
	}
	fm.Normalize()
	node.UpdateMask = fm
	if fm.IsValid(node.Node) {
		_, err = c.nodeService.Update(ctx, node)
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
		Name: s,
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
