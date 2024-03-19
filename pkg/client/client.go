package client

import (
	"context"

	"github.com/amimof/blipblop/api/services/containers/v1"
	"github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/api/services/nodes/v1"
	"github.com/amimof/blipblop/pkg/labels"
	"github.com/amimof/blipblop/services/event"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	//"google.golang.org/grpc/keepalive"
	"io"
	"log"
	"net"
	"os"
	"runtime"
	"sync"
	"time"

	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type RESTClient struct {
}

type Client struct {
	name             string
	conn             *grpc.ClientConn
	nodeService      nodes.NodeServiceClient
	eventService     events.EventServiceClient
	containerService containers.ContainerServiceClient
	//runtime          *RuntimeClient
	mu sync.Mutex
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

func (c *Client) Name() string {
	return c.name
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	err := c.ForgetNode(context.Background(), c.name)
	if err != nil {
		return err
	}
	c.conn.Close()
	return nil
}

func (c *Client) SetContainerNode(ctx context.Context, id, node string) error {
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

func (c *Client) SetContainerState(ctx context.Context, id, state string) error {
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

func (c *Client) SetNodeReady(ctx context.Context, ready bool) error {
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

func (c *Client) UpdateNode(ctx context.Context, node *nodes.Node) error {
	node.Updated = timestamppb.New(time.Now())
	node.Revision = node.Revision + 1
	_, err := c.nodeService.Update(ctx, &nodes.UpdateNodeRequest{Node: node})
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) JoinNode(ctx context.Context, node *nodes.Node) error {
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

func (c *Client) ForgetNode(ctx context.Context, n string) error {
	req := &nodes.ForgetRequest{
		Id: n,
	}
	_, err := c.nodeService.Forget(ctx, req)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) GetContainer(ctx context.Context, id string) (*containers.Container, error) {
	res, err := c.containerService.Get(ctx, &containers.GetContainerRequest{Id: id})
	if err != nil {
		return nil, err
	}
	return res.Container, nil
}

func (c *Client) ListContainers(ctx context.Context) ([]*containers.Container, error) {
	res, err := c.containerService.List(ctx, &containers.ListContainerRequest{Selector: labels.New()})
	if err != nil {
		return nil, err
	}
	return res.Containers, nil
}

func (c *Client) DeleteContainer(ctx context.Context, id string) error {
	_, err := c.containerService.Delete(ctx, &containers.DeleteContainerRequest{Id: id})
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) Publish(ctx context.Context, id string, evt events.EventType) error {
	req := &events.PublishRequest{Event: event.NewEventFor(id, evt)}
	_, err := c.eventService.Publish(ctx, req)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) Subscribe(ctx context.Context) (<-chan *events.Event, <-chan error) {
	evc := make(chan *events.Event)
	errc := make(chan error)
	stream, err := c.eventService.Subscribe(ctx, &events.SubscribeRequest{Id: c.name})
	if err != nil {
		log.Fatalf("subscribe error occurred %s", err.Error())
	}
	go func() {
		for {
			in, err := stream.Recv()
			if err == io.EOF {
				errc <- err
			}
			if err != nil {
				errc <- err
			}
			evc <- in
		}
	}()
	return evc, errc
}

func New(ctx context.Context, server string) (*Client, error) {
	var opts []grpc.DialOption
	retryPolicy := `{
		"methodConfig": [{
		  "name": [{"service": "grpc.examples.echo.Echo"}],
		  "waitForReady": true,
		  "retryPolicy": {
			  "MaxAttempts": 4,
			  "InitialBackoff": ".01s",
			  "MaxBackoff": ".01s",
			  "BackoffMultiplier": 1.0,
			  "RetryableStatusCodes": [ "UNAVAILABLE" ]
		  }
		}]}`
	opts = append(opts, grpc.WithBlock(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDefaultServiceConfig(retryPolicy))
	conn, err := grpc.DialContext(ctx, server, opts...)
	if err != nil {
		return nil, err
	}
	c := &Client{
		conn:             conn,
		nodeService:      nodes.NewNodeServiceClient(conn),
		eventService:     events.NewEventServiceClient(conn),
		containerService: containers.NewContainerServiceClient(conn),
	}
	return c, nil
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
