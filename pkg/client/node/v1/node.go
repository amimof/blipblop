package v1

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"github.com/amimof/voiyd/pkg/condition"
	"github.com/amimof/voiyd/pkg/labels"
	"github.com/amimof/voiyd/pkg/logger"
	"github.com/amimof/voiyd/services/node"

	eventsv1 "github.com/amimof/voiyd/api/services/events/v1"
	nodesv1 "github.com/amimof/voiyd/api/services/nodes/v1"
	typesv1 "github.com/amimof/voiyd/api/types/v1"
)

type CreateOption func(c *clientV1)

func WithLogger(l logger.Logger) CreateOption {
	return func(c *clientV1) {
		c.logger = l
	}
}

func WithClient(client nodesv1.NodeServiceClient) CreateOption {
	return func(c *clientV1) {
		c.Client = client
	}
}

type ClientV1 interface {
	Status() StatusClientV1
	Create(context.Context, *nodesv1.Node, ...CreateOption) error
	Update(context.Context, string, *nodesv1.Node) error
	Get(context.Context, string) (*nodesv1.Node, error)
	Delete(context.Context, string) error
	List(context.Context, ...labels.Label) ([]*nodesv1.Node, error)
	Join(context.Context, *nodesv1.Node) error
	Forget(context.Context, string) error
	Connect(context.Context, string, chan *eventsv1.Event, chan error) error
	Upgrade(context.Context, string, string) error
	UpgradeAll(context.Context, map[string]string, string) error
	Condition(context.Context, ...*typesv1.ConditionReport) error
}

type StatusClientV1 interface {
	Update(context.Context, string, *nodesv1.Status, ...string) error
}

type statusClientV1 struct {
	client nodesv1.NodeServiceClient
}

func (c *clientV1) Status() StatusClientV1 {
	return &statusClientV1{
		client: c.Client,
	}
}

type clientV1 struct {
	Client nodesv1.NodeServiceClient
	id     string
	mu     sync.Mutex
	stream nodesv1.NodeService_ConnectClient
	logger logger.Logger
}

func (c *clientV1) NodeService() nodesv1.NodeServiceClient {
	return c.Client
}

// Update implements StatusClientV1.
func (c *statusClientV1) Update(ctx context.Context, id string, status *nodesv1.Status, path ...string) error {
	// Construct field mask
	mask := &fieldmaskpb.FieldMask{
		Paths: path,
	}

	req := &nodesv1.UpdateStatusRequest{
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
	ctx = metadata.AppendToOutgoingContext(ctx, "voiyd_client_id", c.id)
	_, err := c.Client.Delete(ctx, &nodesv1.DeleteRequest{Id: id})
	if err != nil {
		return err
	}
	return nil
}

func (c *clientV1) Get(ctx context.Context, id string) (*nodesv1.Node, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, "voiyd_client_id", c.id)
	n, err := c.Client.Get(ctx, &nodesv1.GetRequest{Id: id})
	if err != nil {
		return nil, err
	}
	return n.GetNode(), nil
}

func (c *clientV1) List(ctx context.Context, l ...labels.Label) ([]*nodesv1.Node, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, "voiyd_client_id", c.id)
	n, err := c.Client.List(ctx, &nodesv1.ListRequest{})
	if err != nil {
		return nil, err
	}
	return n.Nodes, nil
}

func (c *clientV1) Update(ctx context.Context, id string, node *nodesv1.Node) error {
	ctx = metadata.AppendToOutgoingContext(ctx, "voiyd_client_id", c.id)
	_, err := c.Client.Update(ctx, &nodesv1.UpdateRequest{Id: id, Node: node})
	if err != nil {
		return err
	}
	return nil
}

func (c *clientV1) Create(ctx context.Context, node *nodesv1.Node, opts ...CreateOption) error {
	ctx = metadata.AppendToOutgoingContext(ctx, "voiyd_client_id", c.id)
	_, err := c.Client.Create(ctx, &nodesv1.CreateRequest{Node: node})
	if err != nil {
		return err
	}
	return nil
}

func (c *clientV1) Join(ctx context.Context, node *nodesv1.Node) error {
	ctx = metadata.AppendToOutgoingContext(ctx, "voiyd_client_id", c.id)
	_, err := c.Client.Join(ctx, &nodesv1.JoinRequest{Node: node})
	if err != nil {
		return err
	}
	return nil
}

func (c *clientV1) Forget(ctx context.Context, n string) error {
	ctx = metadata.AppendToOutgoingContext(ctx, "voiyd_client_id", c.id)
	req := &nodesv1.ForgetRequest{
		Id: n,
	}
	_, err := c.Client.Forget(ctx, req)
	if err != nil {
		return err
	}
	return nil
}

func (c *clientV1) Connect(ctx context.Context, nodeName string, receiveChan chan *eventsv1.Event, errChan chan error) error {
	for {
		// Check if the context is already canceled before starting a connection
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Start a new stream connection
		stream, err := c.startStream(ctx, nodeName)
		if err != nil {
			c.logger.Info("error connecting to stream", "error", err)
			time.Sleep(2 * time.Second)
			continue
		}

		c.logger.Info("connected to server stream")

		// Update status once connected
		node, err := c.Get(ctx, nodeName)
		if err != nil {
			return err
		}

		reporter := condition.NewForResource(node)
		err = c.Condition(ctx, reporter.Type(condition.NodeReady).True(condition.ReasonConnected))
		if err != nil {
			return err
		}

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

func (c *clientV1) Upgrade(ctx context.Context, nodeID string, version string) error {
	_, err := c.Client.Upgrade(ctx, &nodesv1.UpgradeRequest{
		NodeId:        nodeID,
		TargetVersion: version,
	})
	return err
}

func (c *clientV1) UpgradeAll(ctx context.Context, selector map[string]string, version string) error {
	_, err := c.Client.Upgrade(ctx, &nodesv1.UpgradeRequest{
		NodeSelector:  selector,
		TargetVersion: version,
	})
	return err
}

func (c *clientV1) Condition(ctx context.Context, reports ...*typesv1.ConditionReport) error {
	for _, r := range reports {
		if _, err := c.Client.Condition(ctx, &typesv1.ConditionRequest{ResourceVersion: node.Version, Report: r}); err != nil {
			return err
		}
	}
	return nil
}

func (c *clientV1) startStream(ctx context.Context, nodeName string) (nodesv1.NodeService_ConnectClient, error) {
	mdCtx := metadata.AppendToOutgoingContext(ctx, "voiyd_node_name", nodeName)
	stream, err := c.Client.Connect(mdCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to create stream: %v", err)
	}
	return stream, nil
}

func (c *clientV1) handleStream(ctx context.Context, stream nodesv1.NodeService_ConnectClient, receiveChan chan<- *eventsv1.Event, errChan chan<- error) error {
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

func (c *clientV1) SendMessage(ctx context.Context, msg *eventsv1.Event) error {
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
		Client: nodesv1.NewNodeServiceClient(conn),
		id:     clientID,
		logger: logger.ConsoleLogger{},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}
