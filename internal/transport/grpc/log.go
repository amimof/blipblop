package grpc

import (
	"context"
	"io"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/amimof/voiyd/internal/app"
	"github.com/amimof/voiyd/pkg/events"

	logsv1 "github.com/amimof/voiyd/api/services/logs/v1"
)

var _ logsv1.LogServiceServer = &LogService{}

type LogService struct {
	logsv1.UnimplementedLogServiceServer
	app *app.LogService
}

func (c *LogService) Register(server *grpc.Server) {
	logsv1.RegisterLogServiceServer(server, c)
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

	logCh := s.app.LogExchange.Subscribe(key)
	defer s.app.LogExchange.Unsubscribe(key, logCh)

	commandCtx := context.Background()

	if err := s.app.SendStartLogsCommand(commandCtx, req); err != nil {
		return err
	}

	defer func() {
		if err := s.app.SendStopLogsCommand(commandCtx, req); err != nil {
			s.app.Logger.Error("error sending stop logs command", "error", err)
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

func (s *LogService) PushLogs(stream logsv1.LogService_PushLogsServer) error {
	ctx := stream.Context()
	seenKeys := make(map[events.LogKey]struct{})

	defer func() {
		for key := range seenKeys {
			s.app.LogExchange.CloseKey(key)
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
		s.app.LogExchange.Publish(entry)

		// We could also add tracing/metrics here.
		select {
		case <-ctx.Done():

			return ctx.Err()
		default:
		}
	}
}

func NewLogService(app *app.LogService) *LogService {
	return &LogService{app: app}
}
