package client

import (
	"context"
	"sync"

	containerv1 "github.com/amimof/blipblop/pkg/client/container/v1"
	eventv1 "github.com/amimof/blipblop/pkg/client/event/v1"
	nodev1 "github.com/amimof/blipblop/pkg/client/node/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type ClientSet struct {
	name              string
	conn              *grpc.ClientConn
	nodeV1Client      *nodev1.NodeV1Client
	containerV1Client *containerv1.ContainerV1Client
	eventV1Client     *eventv1.EventV1Client
	mu                sync.Mutex
}

func (c *ClientSet) NodeV1() *nodev1.NodeV1Client {
	return c.nodeV1Client
}

func (c *ClientSet) ContainerV1() *containerv1.ContainerV1Client {
	return c.containerV1Client
}

func (c *ClientSet) EventV1() *eventv1.EventV1Client {
	return c.eventV1Client
}

func (c *ClientSet) Name() string {
	return c.name
}

func (c *ClientSet) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	err := c.NodeV1().ForgetNode(context.Background(), c.name)
	if err != nil {
		return err
	}
	c.conn.Close()
	return nil
}

func New(ctx context.Context, server string) (*ClientSet, error) {
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
	c := &ClientSet{
		conn:              conn,
		nodeV1Client:      nodev1.NewNodeV1Client(conn),
		containerV1Client: containerv1.NewContainerV1Client(conn),
		eventV1Client:     eventv1.NewEventV1Client(conn),
	}
	return c, nil
}
