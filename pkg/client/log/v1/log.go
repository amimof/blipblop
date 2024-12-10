package log

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	logsv1 "github.com/amimof/blipblop/api/services/logs/v1"
	"github.com/amimof/blipblop/pkg/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type ClientOption func(*ClientV1)

func WithLogger(l logger.Logger) ClientOption {
	return func(c *ClientV1) {
		c.logger = l
	}
}

type ClientV1 struct {
	logger logger.Logger
	client logsv1.LogServiceClient
}

func (c *ClientV1) LogService() logsv1.LogServiceClient {
	return c.client
}

func (c *ClientV1) StreamLogs(ctx context.Context, req *logsv1.SubscribeRequest, receiveChan chan *logsv1.SubscribeResponse, errChan chan error) error {
	for {
		// Check if the context is already canceled before starting a connection
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		// Start a new stream connectio
		stream, err := c.startStream(ctx, req)
		if err != nil {
			errChan <- err
			time.Sleep(2 * time.Second)
			continue
		}

		// Stream handling
		streamErr := c.handleStream(ctx, stream, receiveChan, errChan)

		// Log and retry on transiet errors
		if streamErr != nil {

			// Stream closed due to context cancellation
			if errors.Is(streamErr, context.Canceled) {
				errChan <- err
				return err
			}

			// Backoff reconnect
			time.Sleep(2 * time.Second)
		}
	}
}

func (c *ClientV1) startStream(ctx context.Context, req *logsv1.SubscribeRequest) (logsv1.LogService_SubscribeClient, error) {
	stream, err := c.client.Subscribe(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create stream: %v", err)
	}
	return stream, nil
}

func (c *ClientV1) startCollect(ctx context.Context, nodeName string, containerName string) (logsv1.LogService_LogStreamClient, error) {
	mdCtx := metadata.AppendToOutgoingContext(ctx, "blipblop_node_name", nodeName)
	mdCtx = metadata.AppendToOutgoingContext(mdCtx, "blipblop_container_name", containerName)

	stream, err := c.client.LogStream(mdCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to create stream: %v", err)
	}
	return stream, nil
}

func (c *ClientV1) handleStream(ctx context.Context, stream logsv1.LogService_SubscribeClient, receiveChan chan<- *logsv1.SubscribeResponse, errChan chan<- error) error {
	// Start receiving messages from the server
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			response, err := stream.Recv()
			if err != nil {

				// Handle EOF and retryable gRPC errors
				if errors.Is(err, io.EOF) {
					return io.EOF
				}

				// Transient stream error
				if s, ok := status.FromError(err); ok && isRetryableError(s.Code()) {
					errChan <- err
					return err
				}

				// Non-retryable error
				errChan <- err
				return err
			}
			receiveChan <- response
		}
	}
}

func (c *ClientV1) handleCollect(ctx context.Context, stream logsv1.LogService_LogStreamClient, receiveChan <-chan *logsv1.LogStreamRequest, errChan chan<- error, respChan chan<- *logsv1.LogStreamResponse) error {
	doneCh := make(chan struct{})

	// Goroutine for receiving messages
	go func() {
		for {
			select {
			case <-ctx.Done():
				close(doneCh)
				return
			default:
				response, err := stream.Recv()
				if err != nil {

					// Handle EOF and retryable gRPC errors
					if errors.Is(err, io.EOF) {
						return
					}

					// Transient stream error
					if s, ok := status.FromError(err); ok && isRetryableError(s.Code()) {
						errChan <- err
						return
					}

					// Non-retryable error
					errChan <- err
				}
				// Send received message to chan
				respChan <- response
			}
		}
	}()

	// Goroutine for sending messages
	go func() {
		for {
			select {
			case <-ctx.Done():
				close(doneCh)
				return
			case req, ok := <-receiveChan:
				if !ok {
					return
				}
				if err := stream.Send(req); err != nil {
					errChan <- err
					return
				}
			}
		}
	}()

	<-doneCh

	if err := stream.CloseSend(); err != nil {
		return err
	}

	return nil
}

func isRetryableError(code codes.Code) bool {
	return code == codes.Unavailable || code == codes.ResourceExhausted || code == codes.Internal
}

func (c *ClientV1) LogStream(ctx context.Context, nodeName string, containerName string, receiveChan chan *logsv1.LogStreamRequest, errChan chan error, resChan chan<- *logsv1.LogStreamResponse) error {
	for {
		// Check if the context is already canceled before starting a connection
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		// Start a new stream connection
		stream, err := c.startCollect(ctx, nodeName, containerName)
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}

		// Stream handling
		streamErr := c.handleCollect(ctx, stream, receiveChan, errChan, resChan)

		// Log and retry on transiet errors
		if streamErr != nil {

			// Stream closed due to context cancellation
			if errors.Is(streamErr, context.Canceled) {
				return err
			}

			// Backoff reconnect
			time.Sleep(2 * time.Second)
		}

	}
}

func NewClientV1(conn *grpc.ClientConn, opts ...ClientOption) *ClientV1 {
	c := &ClientV1{
		client: logsv1.NewLogServiceClient(conn),
		logger: logger.ConsoleLogger{},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}
