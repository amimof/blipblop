package v1

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/api/services/nodes/v1"
	"github.com/amimof/blipblop/pkg/labels"
	"github.com/amimof/blipblop/pkg/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

type CreateOption func(c *clientV1)

func WithLogger(l logger.Logger) CreateOption {
	return func(c *clientV1) {
		c.logger = l
	}
}

func WithClient(client nodes.NodeServiceClient) CreateOption {
	return func(c *clientV1) {
		c.Client = client
	}
}

type ClientV1 interface {
	Status() StatusClientV1
	Create(context.Context, *nodes.Node, ...CreateOption) error
	Update(context.Context, string, *nodes.Node) error
	Get(context.Context, string) (*nodes.Node, error)
	Delete(context.Context, string) error
	List(context.Context, ...labels.Label) ([]*nodes.Node, error)
	Join(context.Context, *nodes.Node) error
	Forget(context.Context, string) error
	Connect(context.Context, string, chan *events.Event, chan error) error
}

type StatusClientV1 interface {
	Update(context.Context, string, *nodes.Status, ...string) error
}

type statusClientV1 struct {
	client nodes.NodeServiceClient
}

func (c *clientV1) Status() StatusClientV1 {
	return &statusClientV1{
		client: c.Client,
	}
}

type clientV1 struct {
	Client nodes.NodeServiceClient
	id     string
	mu     sync.Mutex
	stream nodes.NodeService_ConnectClient
	logger logger.Logger
}

func (c *clientV1) NodeService() nodes.NodeServiceClient {
	return c.Client
}

// Update implements StatusClientV1.
func (c *statusClientV1) Update(ctx context.Context, id string, status *nodes.Status, path ...string) error {
	// Construct field mask
	mask := &fieldmaskpb.FieldMask{
		Paths: path,
	}

	req := &nodes.UpdateStatusRequest{
		Id:         id,
		UpdateMask: mask,
		Status:     status,
	}

	_, err := c.client.UpdateStatus(ctx, req)
	if err != nil {
		return err
	}

	return nil
}

func (c *clientV1) Delete(ctx context.Context, id string) error {
	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_client_id", c.id)
	_, err := c.Client.Delete(ctx, &nodes.DeleteRequest{Id: id})
	if err != nil {
		return err
	}
	return nil
}

func (c *clientV1) Get(ctx context.Context, id string) (*nodes.Node, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_client_id", c.id)
	n, err := c.Client.Get(ctx, &nodes.GetRequest{Id: id})
	if err != nil {
		return nil, err
	}
	return n.GetNode(), nil
}

func (c *clientV1) List(ctx context.Context, l ...labels.Label) ([]*nodes.Node, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_client_id", c.id)
	n, err := c.Client.List(ctx, &nodes.ListRequest{})
	if err != nil {
		return nil, err
	}
	return n.Nodes, nil
}

func (c *clientV1) Update(ctx context.Context, id string, node *nodes.Node) error {
	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_client_id", c.id)
	_, err := c.Client.Update(ctx, &nodes.UpdateRequest{Id: id, Node: node})
	if err != nil {
		return err
	}
	return nil
}

func (c *clientV1) Create(ctx context.Context, node *nodes.Node, opts ...CreateOption) error {
	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_client_id", c.id)
	_, err := c.Client.Create(ctx, &nodes.CreateRequest{Node: node})
	if err != nil {
		return err
	}
	return nil
}

func (c *clientV1) Join(ctx context.Context, node *nodes.Node) error {
	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_client_id", c.id)
	_, err := c.Client.Join(ctx, &nodes.JoinRequest{Node: node})
	if err != nil {
		return err
	}
	return nil
}

func (c *clientV1) Forget(ctx context.Context, n string) error {
	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_client_id", c.id)
	req := &nodes.ForgetRequest{
		Id: n,
	}
	_, err := c.Client.Forget(ctx, req)
	if err != nil {
		return err
	}
	return nil
}

func (c *clientV1) Connect(ctx context.Context, nodeName string, receiveChan chan *events.Event, errChan chan error) error {
	for {
		// Check if the context is already canceled before starting a connection
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		// Start a new stream connection
		stream, err := c.startStream(ctx, nodeName)
		if err != nil {
			c.logger.Info("error connecting to stream", "error", err)
			time.Sleep(2 * time.Second)
			continue
		}

		// log.Println("Connected to stream")
		c.logger.Info("connected to stream", "node", nodeName)

		// Stream handling
		streamErr := c.handleStream(ctx, stream, receiveChan, errChan)

		// Log and retry on transiet errors
		if streamErr != nil {

			// Stream closed due to context cancellation
			if errors.Is(streamErr, context.Canceled) {
				return streamErr
			}

			// Backoff reconnect
			c.logger.Error("reconnecting due to stream error", "error", streamErr)
			time.Sleep(2 * time.Second)
		}

	}
}

func (c *clientV1) startStream(ctx context.Context, nodeName string) (nodes.NodeService_ConnectClient, error) {
	mdCtx := metadata.AppendToOutgoingContext(ctx, "blipblop_node_name", nodeName)
	stream, err := c.Client.Connect(mdCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to create stream: %v", err)
	}
	return stream, nil
}

func (c *clientV1) handleStream(ctx context.Context, stream nodes.NodeService_ConnectClient, receiveChan chan<- *events.Event, errChan chan<- error) error {
	// Start receiving messages from the server
	for {
		select {
		case <-ctx.Done():
			return context.Canceled
		default:
			response, err := stream.Recv()
			if err != nil {

				if errors.Is(err, context.Canceled) {
					return context.Canceled
				}

				// Handle EOF and retryable gRPC errors
				if errors.Is(err, io.EOF) {
					return io.EOF
				}

				// Transient stream error
				if s, ok := status.FromError(err); ok && isRetryableError(s.Code()) {
					return fmt.Errorf("transient stream error %s %s: %v", s.Message(), s.Code(), err)
				}

				// Non-retryable error
				errChan <- err
				return err
			}
			// Send received message to chan
			receiveChan <- response
		}
	}
}

func isRetryableError(code codes.Code) bool {
	return code == codes.Unavailable || code == codes.ResourceExhausted || code == codes.Internal
}

func (c *clientV1) SendMessage(ctx context.Context, msg *events.Event) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.stream.Send(msg); err != nil {
		c.stream = nil
		return err
	}

	return nil
}

func NewClientV1(opts ...CreateOption) ClientV1 {
	c := &clientV1{}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func NewClientV1WithConn(conn *grpc.ClientConn, clientID string, opts ...CreateOption) ClientV1 {
	c := &clientV1{
		Client: nodes.NewNodeServiceClient(conn),
		id:     clientID,
		logger: logger.ConsoleLogger{},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}
