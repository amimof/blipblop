package log

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/amimof/blipblop/api/services/logs/v1"
	"github.com/amimof/blipblop/pkg/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ClientOption func(*ClientV1)

func WithLogger(l logger.Logger) ClientOption {
	return func(c *ClientV1) {
		c.logger = l
	}
}

type ClientV1 struct {
	LogClient       logs.LogServiceClient
	CollectClient   logs.LogCollectorServiceClient
	mu              sync.Mutex
	clientStream    logs.LogService_StreamLogsClient
	collectorStream logs.LogCollectorService_CollectLogsClient
	logger          logger.Logger
}

func (c *ClientV1) LogService() logs.LogServiceClient {
	return c.LogClient
}

func (c *ClientV1) StreamLogs(ctx context.Context, containerId string, receiveChan chan *logs.LogResponse, errChan chan error) error {
	for {
		// Check if the context is already canceled before starting a connection
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		// Start a new stream connection
		stream, err := c.startStream(ctx)
		if err != nil {
			c.logger.Info("error connecting to stream", "error", err)
			time.Sleep(2 * time.Second)
			continue
		}

		// log.Println("Connected to stream")
		c.logger.Info("connected to stream")

		if err := stream.Send(&logs.LogRequest{ContainerId: containerId}); err != nil {
			return err
		}

		// Stream handling
		streamErr := c.handleStream(ctx, stream, receiveChan, errChan)

		// Log and retry on transiet errors
		if streamErr != nil {

			// Stream closed due to context cancellation
			if errors.Is(streamErr, context.Canceled) {
				return err
			}

			// Backoff reconnect
			c.logger.Error("reconnecting due to stream error", "error", streamErr)
			time.Sleep(2 * time.Second)
		}
	}
}

func (c *ClientV1) startStream(ctx context.Context) (logs.LogService_StreamLogsClient, error) {
	// mdCtx := metadata.AppendToOutgoingContext(ctx, "blipblop_node_name", nodeName)
	stream, err := c.LogClient.StreamLogs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create stream: %v", err)
	}
	return stream, nil
}

func (c *ClientV1) startCollect(ctx context.Context) (logs.LogCollectorService_CollectLogsClient, error) {
	// mdCtx := metadata.AppendToOutgoingContext(ctx, "blipblop_node_name", nodeName)
	stream, err := c.CollectClient.CollectLogs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create stream: %v", err)
	}
	return stream, nil
}

func (c *ClientV1) handleStream(ctx context.Context, stream logs.LogService_StreamLogsClient, receiveChan chan<- *logs.LogResponse, errChan chan<- error) error {
	// Start receiving messages from the server
	for {
		select {
		case <-ctx.Done():
			err := stream.CloseSend()
			if err != nil {
				fmt.Println("Error closing send channel")
				return err
			}
			return ctx.Err()
		default:
			response, err := stream.Recv()
			if err != nil {

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

func (c *ClientV1) handleCollect(ctx context.Context, stream logs.LogCollectorService_CollectLogsClient, receiveChan <-chan *logs.LogResponse, errChan chan<- error) error {
	// Start receiving messages from the server
	for {
		select {
		case <-ctx.Done():
			return context.Canceled
		default:

			logreq, err := stream.Recv()
			if err != nil {

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

			c.logger.Info("request to stream logs for container", logreq.GetContainerId())
			// Send received message to chan
			// receiveChan <- &logs.LogResponse{LogLine: "this is a log line", Timestamp: time.Now().String()}
			// stream.Send(&logs.LogResponse{LogLine: "this is a log line", Timestamp: time.Now().String()})
			for res := range receiveChan {
				stream.Send(res)
			}
		}
	}
}

func isRetryableError(code codes.Code) bool {
	return code == codes.Unavailable || code == codes.ResourceExhausted || code == codes.Internal
}

func (c *ClientV1) ConnectToLogService(ctx context.Context, containerId string, receiveChan chan *logs.LogResponse, errChan chan error) error {
	for {

		// Check if the context is already canceled before starting a connection
		select {
		case <-ctx.Done():
			return nil
		// case l := <-receiveChan:
		// 	stream.Send(l)
		default:
		}

		// Start a new stream connection
		stream, err := c.startCollect(ctx)
		if err != nil {
			c.logger.Info("error connecting to stream", "error", err)
			time.Sleep(2 * time.Second)
			continue
		}

		// log.Println("Connected to stream")
		c.logger.Info("connected to stream")

		// Stream handling
		streamErr := c.handleCollect(ctx, stream, receiveChan, errChan)

		// Log and retry on transiet errors
		if streamErr != nil {

			// Stream closed due to context cancellation
			if errors.Is(streamErr, context.Canceled) {
				return err
			}

			// Backoff reconnect
			c.logger.Error("reconnecting due to stream error", "error", streamErr)
			time.Sleep(2 * time.Second)
		}

	}
}

func NewClientV1(conn *grpc.ClientConn, opts ...ClientOption) *ClientV1 {
	c := &ClientV1{
		LogClient:     logs.NewLogServiceClient(conn),
		CollectClient: logs.NewLogCollectorServiceClient(conn),
		logger:        logger.ConsoleLogger{},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}
