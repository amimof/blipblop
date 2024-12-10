package log

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync"

	logsv1 "github.com/amimof/blipblop/api/services/logs/v1"
	"github.com/amimof/blipblop/pkg/events"
	"github.com/amimof/blipblop/pkg/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
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
	logsv1.UnimplementedLogServiceServer
	logger   logger.Logger
	exchange *events.Exchange
	mu       sync.Mutex

	nodeStreams   map[string]logsv1.LogService_LogStreamServer
	clientStreams map[string]map[string]chan *logsv1.LogItem
	controlCh     map[string]chan bool
}

func (s *LogService) Register(server *grpc.Server) error {
	server.RegisterService(&logsv1.LogService_ServiceDesc, s)
	return nil
}

func (s *LogService) LogStream(stream logsv1.LogService_LogStreamServer) error {
	ctx := stream.Context()
	nodeName := getNodeNameFromContext(ctx)
	containerId := getContainerIdFromContext(ctx)

	log.Println("node name", nodeName)
	log.Println("container name", containerId)

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

	s.mu.Lock()
	s.nodeStreams[nodeName] = stream
	s.clientStreams[nodeName] = make(map[string]chan *logsv1.LogItem, 100)
	s.clientStreams[nodeName][containerId] = make(chan *logsv1.LogItem, 100)
	s.controlCh[nodeName] = make(chan bool, 1)
	s.mu.Unlock()

	controlCh := s.controlCh[nodeName]

	// Cleanup
	defer func() {
		s.mu.Lock()
		delete(s.nodeStreams, nodeName)
		s.mu.Unlock()
		close(s.controlCh[nodeName])
	}()

	go func() {
		for signal := range controlCh {
			log.Printf("Sending signal %t", signal)
			if err := stream.Send(&logsv1.LogStreamResponse{Start: signal}); err != nil {
				log.Printf("error sending log response signal %s: %t", err, signal)
			}
		}
	}()

	for {
		select {
		case <-stream.Context().Done():
			log.Println("Context done, quitting out")
			return nil
		default:

			req, err := stream.Recv()
			if err == io.EOF {
				// n.logger.Error("error receving from stream, stream probably closed", "node", nodeName)
				log.Println("error receving from stream, stream probably closed")
				return err
			}

			if err != nil {
				// n.logger.Error("error receving data from stream", "error", err, "node", nodeName)
				log.Println("error receving data from stream", err)
				return err
			}

			logMessage := &logsv1.LogItem{
				LogLine:   req.GetLog().GetLogLine(),
				Timestamp: req.GetLog().GetTimestamp(),
			}

			log.Println("Received log line from node, forwarding to clients", nodeName, containerId)
			s.clientStreams[nodeName][containerId] <- logMessage

			// for _, s := range s.streams {
			// 	if err := s.Send(msg); err != nil {
			// 		log.Println("Error sending logline to client", err)
			// 	}
			// }

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

func (s *LogService) Subscribe(req *logsv1.SubscribeRequest, stream logsv1.LogService_SubscribeServer) error {
	log.Printf("Client %s subscribed to logs", req.ClientId)

	nodeId := req.GetNodeId()
	containerId := req.GetContainerId()

	controlCh := s.getLogControlChannel(nodeId)
	if controlCh == nil {
		return fmt.Errorf("no agent found for ID %s", nodeId)
	}

	// Signal node to start streaming
	controlCh <- true
	defer func() {
		controlCh <- false
	}()

	clientStream := s.clientStreams[nodeId][containerId]

	for logMessage := range clientStream {
		log.Println("Got message that i need to send forward")
		res := &logsv1.SubscribeResponse{Log: logMessage}
		if err := stream.Send(res); err != nil {
			log.Printf("error sending log to client %s: %v", req.GetClientId(), err)
			return err
		}
	}

	return nil
}

func (s *LogService) getLogControlChannel(nodeId string) chan bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.controlCh[nodeId]; !ok {
		s.controlCh[nodeId] = make(chan bool, 1)
	}
	return s.controlCh[nodeId]
}

func getNodeNameFromContext(ctx context.Context) string {
	return getKeyFromContext(ctx, "blipblop_node_name")
}

func getContainerIdFromContext(ctx context.Context) string {
	return getKeyFromContext(ctx, "blipblop_container_name")
}

func getKeyFromContext(ctx context.Context, key string) string {
	var keyValue string
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if k, ok := md[key]; ok && len(k) > 0 {
			keyValue = k[0]
		}
	}
	return keyValue
}

func NewService(opts ...NewServiceOption) *LogService {
	s := &LogService{
		logger: logger.ConsoleLogger{},
		// streams: make(map[string]logsv1.LogService_StreamLogsServer),
		// nodeStreams: make(map[string]map[string]logs.LogCollectorService_CollectLogsServer),
		// nodeStreams: make(map[string]map[string]nodeStream),
		nodeStreams:   make(map[string]logsv1.LogService_LogStreamServer, 10),
		clientStreams: make(map[string]map[string]chan *logsv1.LogItem, 10),
		controlCh:     make(map[string]chan bool, 1),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}
