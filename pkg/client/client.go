package client

import (
	"context"
	"errors"
	"github.com/amimof/blipblop/api/services/containers/v1"
	"github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/api/services/nodes/v1"
	"github.com/amimof/blipblop/internal/models"
	"github.com/amimof/blipblop/internal/services"
	"google.golang.org/grpc"
	"io"
	"log"
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

func NewLocalClient(nodeService *services.NodeService, eventService *services.EventService, containerService *services.ContainerService) *LocalClient {
	return &LocalClient{
		nodeService:      nodeService,
		eventService:     eventService,
		containerService: containerService,
	}
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

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	err := c.ForgetNode(context.Background())
	if err != nil {
		return err
	}
	c.conn.Close()
	return nil
}

func (c *Client) JoinNode(ctx context.Context, name string) error {
	c.name = name
	n := &nodes.JoinRequest{
		Node: &nodes.Node{
			Id: c.name,
		},
	}
	res, err := c.nodeService.Join(ctx, n)
	if err != nil {
		return err
	}
	if res.Status == nodes.Status_JoinFail {
		return errors.New("unable to join node")
	}
	return nil
}

func (c *Client) ForgetNode(ctx context.Context) error {
	req := &nodes.ForgetRequest{
		Node: c.name,
	}
	res, err := c.nodeService.Forget(ctx, req)
	if err != nil {
		return err
	}
	if res.Status == nodes.Status_ForgetFail {
		return errors.New("unable to forget node")
	}
	return nil
}

func (c *Client) GetContainer(ctx context.Context, id string) (*models.Container, error) {
	res, err := c.containerService.GetTest(ctx, &containers.GetContainerRequest{Id: id})
	if err != nil {
		return nil, err
	}
	container := &models.Container{
		Name:   &res.Container.Id,
		Image:  &res.Container.Image,
		Labels: res.Container.Labels,
	}
	return container, nil
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
