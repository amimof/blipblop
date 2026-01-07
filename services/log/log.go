// Package log implements the log service
package log

import (
	"context"
	"io"

	logsv1 "github.com/amimof/voiyd/api/services/logs/v1"
	"github.com/amimof/voiyd/pkg/events"
	"github.com/amimof/voiyd/pkg/logger"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

const Version string = "log/v1"

type NewServiceOption func(s *LogService)

func WithLogger(l logger.Logger) NewServiceOption {
	return func(s *LogService) {
		s.logger = l
	}
}

func WithExchange(e *events.Exchange) NewServiceOption {
	return func(s *LogService) {
		s.exchange = e
	}
}

type LogService struct {
	logsv1.UnimplementedLogServiceServer
	logger      logger.Logger
	exchange    *events.Exchange
	logExchange *events.LogExchange
}

func (s *LogService) Register(server *grpc.Server) error {
	server.RegisterService(&logsv1.LogService_ServiceDesc, s)
	return nil
}

func (s *LogService) sendStartLogsCommand(ctx context.Context, req *logsv1.TailLogRequest) error {
	err := s.exchange.Publish(ctx, events.NewEvent(events.TailLogsStart, req))
	if err != nil {
		s.logger.Error("error publishing TailLogStart event", "nodeID", req.GetNodeId(), "containerID", req.GetTaskId(), "tail?", req.GetWatch())
		return err
	}
	return nil
}

func (s *LogService) sendStopLogsCommand(ctx context.Context, req *logsv1.TailLogRequest) error {
	err := s.exchange.Publish(ctx, events.NewEvent(events.TailLogsStop, req))
	if err != nil {
		s.logger.Error("error publishing TailLogStart event", "nodeID", req.GetNodeId(), "containerID", req.GetTaskId(), "tail?", req.GetWatch())
		return err
	}
	return nil
}

// TailLogs publishes TailLogStart to subscribers requesting them to start sending entries to the server.
// TailLogs will then forward the entires, fanning them out to the client(s) making the request in the first place.
func (s *LogService) TailLogs(req *logsv1.TailLogRequest, srv logsv1.LogService_TailLogsServer) error {
	ctx := srv.Context()

	// Append session id if missing in original request
	if len(req.GetSessionId()) == 0 {
		req.SessionId = uuid.New().String()
	}

	// Subscribe and get the log channel
	key := events.LogKey{
		NodeID:    req.GetNodeId(),
		TaskID:    req.GetTaskId(),
		SessionID: req.GetSessionId(),
	}

	logCh := s.logExchange.Subscribe(key)
	defer s.logExchange.Unsubscribe(key, logCh)

	commandCtx := context.Background()

	if err := s.sendStartLogsCommand(commandCtx, req); err != nil {
		return err
	}

	defer func() {
		if err := s.sendStopLogsCommand(commandCtx, req); err != nil {
			s.logger.Error("error sending stop logs command", "error", err)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case entry, ok := <-logCh:
			if !ok {
				return nil
			}
			if err := srv.Send(entry); err != nil {
				return err
			}
		}
	}
}

// PushLogs can be used by clients to push log entries to the server which fans them out to subscribers on the exchange.
func (s *LogService) PushLogs(stream logsv1.LogService_PushLogsServer) error {
	ctx := stream.Context()

	seenKeys := make(map[events.LogKey]struct{})

	defer func() {
		for key := range seenKeys {
			s.logExchange.CloseKey(key)
		}
	}()

	for {
		entry, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				return stream.SendAndClose(&emptypb.Empty{})
			}
			// Here we might log the error; returning ends the stream.
			// For transient node issues, the node will reconnect.
			st, ok := status.FromError(err)
			if ok && st.Code() == codes.Canceled {
				return nil
			}

			return err
		}

		key := events.LogKey{
			NodeID:    entry.GetNodeId(),
			TaskID:    entry.GetTaskId(),
			SessionID: entry.GetSessionId(),
		}

		if key.NodeID != "" && key.TaskID != "" {
			seenKeys[key] = struct{}{}
		}

		// Fan out to all subscribers
		s.logExchange.Publish(entry)

		// We could also add tracing/metrics here.
		select {
		case <-ctx.Done():

			return ctx.Err()
		default:
		}
	}
}

// NewService initializes and returns a LogService instance
func NewService(opts ...NewServiceOption) *LogService {
	s := &LogService{
		logger:      logger.ConsoleLogger{},
		logExchange: &events.LogExchange{},
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}
