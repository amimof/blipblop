package log

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync"

	"github.com/amimof/blipblop/api/services/logs/v1"
	"github.com/amimof/blipblop/pkg/events"
	"github.com/amimof/blipblop/pkg/logger"
	"github.com/google/uuid"
	"google.golang.org/grpc"
)

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
	logs.UnimplementedLogCollectorServiceServer
	logs.UnimplementedLogServiceServer
	// local    nodesv1.NodeServiceClient
	logger      logger.Logger
	exchange    *events.Exchange
	streams     map[string]logs.LogService_StreamLogsServer
	nodeStreams map[string]logs.LogCollectorService_CollectLogsServer
	mu          sync.Mutex
}

func (s *LogService) Register(server *grpc.Server) error {
	server.RegisterService(&logs.LogService_ServiceDesc, s)
	server.RegisterService(&logs.LogCollectorService_ServiceDesc, s)
	// containers.RegisterLogServiceServer(server, c)
	return nil
}

func (s *LogService) CollectLogs(stream logs.LogCollectorService_CollectLogsServer) error {
	ctx := stream.Context()

	// TESTING Periodically send message to clients
	// go func() {
	// 	res := &logs.LogResponse{LogLine: "this is a log statement", Timestamp: time.Now().String()}
	//
	// 	for {
	// 		stream.Send(res)
	// 		time.Sleep(3 * time.Second)
	// 	}
	// }()

	// TESTING request logs from a container
	// go func() {
	// 	time.Sleep(3 * time.Second)
	// 	res := &logs.LogRequest{ContainerId: "nginx2"}
	// 	stream.Send(res)
	// }()

	// nodeName := uuid.New().String()
	nodeName := "bbnode"

	s.mu.Lock()
	s.nodeStreams[nodeName] = stream
	s.mu.Unlock()

	for {
		select {
		case <-ctx.Done():
			log.Println("Client is done streaming logs")
			return ctx.Err()
		default:
			msg, err := stream.Recv()
			if err == io.EOF {
				// n.logger.Error("error receving from stream, stream probably closed", "node", nodeName)
				log.Println("error receving from stream, stream probably closed")
				return nil
			}

			if err != nil {
				// n.logger.Error("error receving data from stream", "error", err, "node", nodeName)
				log.Println("error receving data from stream", err)
				return err
			}

			log.Println("Received log line from agent", msg.LogLine)

			for _, s := range s.streams {
				if err := s.Send(msg); err != nil {
					log.Println("Error sending logline to client", err)
				}
			}

			// if err := stream.Send(msg); err != nil {
			// 	return err
			// }A
			// err = n.exchange.Publish(ctx, &eventsv1.PublishRequest{Event: msg})
			// if err != nil {
			// 	return err
			// }
		}
	}
}

func (s *LogService) StreamLogs(stream logs.LogService_StreamLogsServer) error {
	ctx := stream.Context()

	// TESTING Periodically send message to clients
	// go func() {
	// 	res := &logs.LogResponse{LogLine: "this is a log statement", Timestamp: time.Now().String()}
	// 	for {
	// 		stream.Send(res)
	// 		time.Sleep(3 * time.Second)
	// 	}
	// }()

	clientName := uuid.New().String()

	s.mu.Lock()
	s.streams[clientName] = stream
	s.mu.Unlock()

	// Remove client
	defer func() {
		fmt.Printf("cleaning up after client disconnect %s\n", clientName)
		delete(s.streams, clientName)
	}()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("Client done?")
			switch ctx.Err() {
			case context.DeadlineExceeded:
				log.Println("context timout exceeded")
				return ctx.Err()
			case context.Canceled:
				log.Println("Client canceled")
				return ctx.Err()
			default:
				log.Println("Client is done streaming logs for unknown reason")
				return ctx.Err()
			}
		default:
			msg, err := stream.Recv()
			if err == io.EOF {
				// n.logger.Error("error receving from stream, stream probably closed", "node", nodeName)
				log.Println("error receving from stream, stream probably closed")
				return nil
			}

			if err != nil {
				// n.logger.Error("error receving data from stream", "error", err, "node", nodeName)
				log.Println("error receving data from stream", err)
				return err
			}

			log.Println("Received log request from client", msg.ContainerId)

			// Find the agents tream
			agentStream, ok := s.nodeStreams["bbnode"]
			if !ok {
				return fmt.Errorf("no agent connected with id", "a")
			}

			// Forward request to agent
			err = agentStream.Send(msg)

			// if err := stream.Send(msg); err != nil {
			// 	return err
			// }A
			// err = n.exchange.Publish(ctx, &eventsv1.PublishRequest{Event: msg})
			// if err != nil {
			// 	return err
			// }
		}
	}
}

func NewService(opts ...NewServiceOption) *LogService {
	s := &LogService{
		logger:      logger.ConsoleLogger{},
		streams:     make(map[string]logs.LogService_StreamLogsServer),
		nodeStreams: make(map[string]logs.LogCollectorService_CollectLogsServer),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}
