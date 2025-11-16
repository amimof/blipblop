// Package log provides a client interface to interact with logs
package log

import (
	"context"
	"sync"

	logsv1 "github.com/amimof/blipblop/api/services/logs/v1"
	"github.com/amimof/blipblop/pkg/logger"
	"google.golang.org/grpc"
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
	mu     sync.Mutex
}

type Stream struct {
	stream logsv1.LogService_PushLogsClient
}

func (s *Stream) Send(entry *logsv1.LogEntry) error {
	return s.stream.Send(entry)
}

func (s *Stream) Close() error {
	return s.stream.CloseSend()
}

func (c *ClientV1) Stream(ctx context.Context) (*Stream, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	stream, err := c.client.PushLogs(ctx)
	if err != nil {
		return nil, err
	}

	return &Stream{stream: stream}, nil
}

func (c *ClientV1) TailLogs(ctx context.Context, req *logsv1.TailLogRequest) (logsv1.LogService_TailLogsClient, error) {
	return c.client.TailLogs(ctx, req)
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
