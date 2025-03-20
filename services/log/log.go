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

	nodeStreams map[string]logsv1.LogService_LogStreamServer
	// clientStreams map[string]map[string]map[string]chan *logsv1.LogItem
	clientStreams map[string]map[string]map[string]chan *logsv1.LogItem
	controlCh     map[string]chan *logsv1.LogStreamResponse
}

func (s *LogService) Register(server *grpc.Server) error {
	server.RegisterService(&logsv1.LogService_ServiceDesc, s)
	return nil
}

func (s *LogService) LogStream(stream logsv1.LogService_LogStreamServer) error {
	ctx := stream.Context()
	nodeName := getNodeNameFromContext(ctx)
	containerId := getContainerIdFromContext(ctx)

	s.mu.Lock()
	s.nodeStreams[nodeName] = stream
	s.clientStreams[nodeName] = make(map[string]map[string]chan *logsv1.LogItem, 100)
	s.clientStreams[nodeName][containerId] = make(map[string]chan *logsv1.LogItem, 100)
	s.controlCh[nodeName] = make(chan *logsv1.LogStreamResponse, 1)
	s.mu.Unlock()

	controlCh := s.controlCh[nodeName]

	// Cleanup
	defer func() {
		s.mu.Lock()
		delete(s.nodeStreams, nodeName)
		s.mu.Unlock()
		close(s.controlCh[nodeName])
	}()

	// Control signal
	go func() {
		for res := range controlCh {
			if err := stream.Send(res); err != nil {
				s.logger.Error("error sending log stream response", "res", res, "error", err)
			}
		}
	}()

	// Main loop
	for {
		select {
		case <-stream.Context().Done():
			log.Println("Context done, quitting out")
			return nil
		default:

			req, err := stream.Recv()
			if err == io.EOF {
				s.logger.Error("error receving from stream, stream probably closed", "error", err)
				return err
			}

			if err != nil {
				s.logger.Error("error receving data from stream", err)
				return err
			}

			logMessage := &logsv1.LogItem{
				LogLine:   req.GetLog().GetLogLine(),
				Timestamp: req.GetLog().GetTimestamp(),
			}

			s.logger.Debug("Received log line from node, forwarding to clients", "node", nodeName, "container", containerId)
			for _, out := range s.clientStreams[nodeName][containerId] {
				out <- logMessage
			}

		}
	}
}

func (s *LogService) Subscribe(req *logsv1.SubscribeRequest, stream logsv1.LogService_SubscribeServer) error {
	if len(req.GetClientId()) == 0 {
		return fmt.Errorf("cannot subscribe with empty clientId")
	}

	s.logger.Debug("client subscribed to logs", "clientId", req.GetClientId())

	nodeId := req.GetNodeId()
	containerId := req.GetContainerId()
	clientId := req.GetClientId()

	controlCh := s.getLogControlChannel(nodeId)
	if controlCh == nil {
		return fmt.Errorf("no agent found for ID %s", nodeId)
	}

	s.clientStreams[nodeId] = map[string]map[string]chan *logsv1.LogItem{}
	s.clientStreams[nodeId][containerId] = map[string]chan *logsv1.LogItem{}
	s.clientStreams[nodeId][containerId][clientId] = make(chan *logsv1.LogItem)
	clientStream := s.clientStreams[nodeId][containerId][clientId]

	// Signal node to start streaming
	// controlCh <- true
	s.signalNodeStart(nodeId, containerId)
	defer func() {
		s.mu.Lock()
		delete(s.clientStreams[nodeId][containerId], clientId)
		s.mu.Unlock()
		s.signalNodeStart(nodeId, containerId)
		close(clientStream)
	}()

	for logMessage := range clientStream {
		res := &logsv1.SubscribeResponse{Log: logMessage}
		if err := stream.Send(res); err != nil {
			s.logger.Error("error sending log to client", "clientId", req.GetClientId(), "error", err)
			return err
		}
	}

	return nil
}

func (s *LogService) signalNodeStart(nodeId, containerId string) {
	controlCh := s.getLogControlChannel(nodeId)
	res := &logsv1.LogStreamResponse{ContainerId: containerId, NodeId: nodeId, Start: false}

	if controlCh == nil {
		return
	}

	if _, ok := s.clientStreams[nodeId]; !ok {
		res.Start = false
		controlCh <- res
		return
	}

	if _, ok := s.clientStreams[nodeId][containerId]; !ok {
		res.Start = false
		controlCh <- res
		return
	}

	if len(s.clientStreams[nodeId][containerId]) == 0 {
		res.Start = false
		controlCh <- res
		return
	}

	res.Start = true
	controlCh <- res
}

func (s *LogService) getLogControlChannel(nodeId string) chan *logsv1.LogStreamResponse {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.controlCh[nodeId]; !ok {
		s.controlCh[nodeId] = make(chan *logsv1.LogStreamResponse, 1)
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
		logger:        logger.ConsoleLogger{},
		nodeStreams:   make(map[string]logsv1.LogService_LogStreamServer, 10),
		clientStreams: make(map[string]map[string]map[string]chan *logsv1.LogItem, 10),
		controlCh:     make(map[string]chan *logsv1.LogStreamResponse, 1),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}
