package client

import (
	"context"
	"github.com/amimof/blipblop/api/services/containers/v1"
	"github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/api/services/nodes/v1"
	"github.com/amimof/blipblop/internal/models"
	"github.com/amimof/blipblop/internal/services"
	"github.com/amimof/blipblop/pkg/labels"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
	"io"
	"log"
	"net"
	"os"
	"runtime"
	"sync"
)

type RESTClient struct {
}

type Client struct {
	name             string
	conn             *grpc.ClientConn
	nodeService      nodes.NodeServiceClient
	eventService     events.EventServiceClient
	containerService containers.ContainerServiceClient
	runtime          *RuntimeClient
	mu               sync.Mutex
}

type LocalClient struct {
	nodeService      *services.NodeService
	eventService     *services.EventService
	containerService *services.ContainerService
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
			i = append(i, addr.String())
		}
	}
	return i
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

func (c *Client) JoinNode(ctx context.Context, node *models.Node) error {
	c.name = *node.Name
	n := &nodes.JoinRequest{
		Node: &nodes.Node{
			Name:     *node.Name,
			Labels:   node.Labels,
			Created:  timestamppb.New(node.Created),
			Updated:  timestamppb.New(node.Updated),
			Revision: node.Revision,
			Status: &nodes.Status{
				Ips:      node.Status.IPs,
				Hostname: node.Status.HostName,
				Os:       node.Status.Os,
				Arch:     node.Status.Arch,
			},
		},
	}
	_, err := c.nodeService.Join(ctx, n)
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

func (c *Client) GetContainer(ctx context.Context, id string) (*models.Container, error) {
	res, err := c.containerService.Get(ctx, &containers.GetContainerRequest{Id: id})
	if err != nil {
		return nil, err
	}
	container := &models.Container{
		Metadata: models.Metadata{
			Name:     &res.Container.Name,
			Labels:   res.Container.Labels,
			Created:  res.Container.Created.AsTime(),
			Updated:  res.Container.Updated.AsTime(),
			Revision: res.Container.Revision,
		},
		Config: &models.ContainerConfig{
			Image: &res.Container.Config.Image,
		},
	}
	return container, nil
}

func (c *Client) ListContainers(ctx context.Context) ([]*models.Container, error) {
	var ctrns []*models.Container
	res, err := c.containerService.List(ctx, &containers.ListContainerRequest{Selector: labels.New()})
	if err != nil {
		return ctrns, err
	}
	for _, ctr := range res.Containers {
		ctrns = append(ctrns, &models.Container{
			Metadata: models.Metadata{
				Name:     &ctr.Name,
				Labels:   ctr.Labels,
				Revision: ctr.Revision,
				Created:  ctr.Created.AsTime(),
				Updated:  ctr.Updated.AsTime(),
			},
			Config: &models.ContainerConfig{
				Image: &ctr.Config.Image,
			},
		})
	}
	return ctrns, nil
}

func (c *Client) DeleteContainer(ctx context.Context, id string) error {
	_, err := c.containerService.Delete(ctx, &containers.DeleteContainerRequest{Id: id})
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

func (l *LocalClient) NodeService() *services.NodeService {
	return l.nodeService
}

func (l *LocalClient) EventService() *services.EventService {
	return l.eventService
}

func (l *LocalClient) ContainerService() *services.ContainerService {
	return l.containerService
}

func (l *LocalClient) Publish(ctx context.Context, e *events.Event) error {
	req := &events.PublishRequest{
		Event: e,
	}
	_, err := l.EventService().Publish(ctx, req)
	if err != nil {
		return err
	}
	return nil
}

func New(server string) (*Client, error) {
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithBlock(), grpc.WithInsecure())
	conn, err := grpc.Dial(server, opts...)
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
func NewNodeFromEnv(s string) *models.Node {
	hostname, _ := os.Hostname()
	n := &models.Node{
		Metadata: models.Metadata{
			Name: &s,
		},
		Status: &models.NodeStatus{
			IPs:      getIpAddressesAsString(),
			HostName: hostname,
			Arch:     runtime.GOARCH,
			Os:       runtime.GOOS,
		},
	}
	return n
}

func NewLocalClient(nodeService *services.NodeService, eventService *services.EventService, containerService *services.ContainerService) *LocalClient {
	return &LocalClient{
		nodeService:      nodeService,
		eventService:     eventService,
		containerService: containerService,
	}
}
