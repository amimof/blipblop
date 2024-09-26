package client

import (
	"context"
	"sync"
	"time"

	containerv1 "github.com/amimof/blipblop/pkg/client/container/v1"
	eventv1 "github.com/amimof/blipblop/pkg/client/event/v1"
	nodev1 "github.com/amimof/blipblop/pkg/client/node/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

type ClientSet struct {
	name              string
	conn              *grpc.ClientConn
	nodeV1Client      *nodev1.ClientV1
	containerV1Client *containerv1.ClientV1
	eventV1Client     *eventv1.ClientV1
	mu                sync.Mutex
}

func (c *ClientSet) NodeV1() *nodev1.ClientV1 {
	return c.nodeV1Client
}

func (c *ClientSet) ContainerV1() *containerv1.ClientV1 {
	return c.containerV1Client
}

func (c *ClientSet) EventV1() *eventv1.ClientV1 {
	return c.eventV1Client
}

func (c *ClientSet) Name() string {
	return c.name
}

func (c *ClientSet) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	err := c.NodeV1().Forget(context.Background(), c.name)
	if err != nil {
		return err
	}
	c.conn.Close()
	return nil
}

func New(server string, opts ...grpc.DialOption) (*ClientSet, error) {
	// Define connection backoff policy
	backoffConfig := backoff.Config{
		BaseDelay:  time.Second,       // Initial delay before retry
		Multiplier: 1.6,               // Multiplier for successive retries
		MaxDelay:   120 * time.Second, // Maximum delay
	}

	// Define keepalive parameters
	keepAliveParams := keepalive.ClientParameters{
		Time:                10 * time.Minute, // Ping the server if no activity
		Timeout:             20 * time.Second, // Timeout for server response
		PermitWithoutStream: true,             // Ping even without active streams
	}

	// Default options
	defaultOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepAliveParams),
		grpc.WithConnectParams(
			grpc.ConnectParams{
				Backoff:           backoffConfig,
				MinConnectTimeout: 20 * time.Second,
			},
		),
		grpc.WithBlock(),
	}

	// Allow passing in custom dial options
	opts = append(defaultOpts, opts...)

	conn, err := grpc.Dial(server, opts...)
	if err != nil {
		return nil, err
	}
	c := &ClientSet{
		conn:              conn,
		nodeV1Client:      nodev1.NewClientV1(conn),
		containerV1Client: containerv1.NewClientV1(conn),
		eventV1Client:     eventv1.NewClientV1(conn),
	}
	return c, nil
}
