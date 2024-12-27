package event

import (
	"context"
	"errors"

	eventsv1 "github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/pkg/eventsv2"
	"github.com/amimof/blipblop/pkg/logger"
	"github.com/amimof/blipblop/pkg/repository"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

var ErrClientExists = errors.New("client already exists")

type NewServiceOption func(s *EventService)

func WithLogger(l logger.Logger) NewServiceOption {
	return func(s *EventService) {
		s.logger = l
	}
}

func WithExchange(e *eventsv2.Exchange) NewServiceOption {
	return func(s *EventService) {
		s.exchange = e
	}
}

type EventService struct {
	eventsv1.UnimplementedEventServiceServer
	local    eventsv1.EventServiceClient
	logger   logger.Logger
	exchange *eventsv2.Exchange
}

func (n *EventService) Create(ctx context.Context, req *eventsv1.CreateEventRequest) (*eventsv1.CreateEventResponse, error) {
	return n.local.Create(ctx, req)
}

func (n *EventService) Get(ctx context.Context, req *eventsv1.GetEventRequest) (*eventsv1.GetEventResponse, error) {
	return n.local.Get(ctx, req)
}

func (n *EventService) Delete(ctx context.Context, req *eventsv1.DeleteEventRequest) (*eventsv1.DeleteEventResponse, error) {
	return n.local.Delete(ctx, req)
}

func (n *EventService) List(ctx context.Context, req *eventsv1.ListEventRequest) (*eventsv1.ListEventResponse, error) {
	return n.local.List(ctx, req)
}

func (n *EventService) Register(server *grpc.Server) error {
	server.RegisterService(&eventsv1.EventService_ServiceDesc, n)
	// eventsv1.RegisterEventServiceServer(server, n)
	return nil
}

func (s *EventService) Subscribe(req *eventsv1.SubscribeRequest, stream eventsv1.EventService_SubscribeServer) error {
	// Identify the client
	ctx := stream.Context()
	clientId := req.ClientId
	peer, _ := peer.FromContext(ctx)

	eventChan := s.exchange.Subscribe(ctx, eventsv2.ALL...)

	s.logger.Debug("client connected", "clientId", clientId, "address", peer.Addr.String())

	go func() {
		for {
			select {
			case n := <-eventChan:

				s.logger.Info("forwarding event from client", "eventType", n.GetType().String(), "objectId", n.GetObjectId(), "eventId", n.GetMeta().GetName(), "clientId", req.ClientId)
				err := stream.Send(n)
				if err != nil {
					s.logger.Error("unable to emit event to clients", "error", err, "eventType", n.GetType().String(), "objectId", n.GetObjectId(), "eventId", n.GetMeta().GetName(), "clientId", req.ClientId)
					return
				}
			case <-ctx.Done():
				s.logger.Info("client disconnected", "clientId", req.ClientId)

				// Get node name from context
				if md, ok := metadata.FromIncomingContext(ctx); ok {
					if nodeName, ok := md["blipblop_node_name"]; ok && len(nodeName) > 0 {
						_, err := s.Publish(ctx, &eventsv1.PublishRequest{Event: &eventsv1.Event{ObjectId: nodeName[0], Type: eventsv1.EventType_NodeForget}})
						if err != nil {
							s.logger.Error("error publishing event", "error", err)
						}
					}
				}
				return
			}
		}
	}()

	<-ctx.Done()
	return nil
}

func (s *EventService) Publish(ctx context.Context, req *eventsv1.PublishRequest) (*eventsv1.PublishResponse, error) {
	s.logger.Info("Publishing event")
	err := s.exchange.Publish(ctx, req.GetEvent().GetType(), req.GetEvent())
	if err != nil {
		return nil, err
	}
	res, err := s.local.Create(ctx, &eventsv1.CreateEventRequest{Event: req.GetEvent()})
	if err != nil {
		return nil, err
	}
	return &eventsv1.PublishResponse{Event: res.GetEvent()}, nil
}

func NewService(repo repository.EventRepository, opts ...NewServiceOption) *EventService {
	s := &EventService{
		logger: logger.ConsoleLogger{},
	}
	for _, opt := range opts {
		opt(s)
	}

	s.local = &local{
		repo: repo,
	}
	return s
}
