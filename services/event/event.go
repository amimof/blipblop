package event

import (
	"context"
	"errors"
	"sync"

	"github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/api/types/v1"
	"github.com/amimof/blipblop/pkg/logger"
	"github.com/amimof/blipblop/pkg/repository"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
)

var ErrClientExists = errors.New("client already exists")

type NewServiceOption func(s *EventService)

func WithLogger(l logger.Logger) NewServiceOption {
	return func(s *EventService) {
		s.logger = l
	}
}

type EventService struct {
	// channel map[string][]chan *events.Event
	channel map[string]chan *events.Event
	// channel1 map[string]events.EventService_SubscribeServer
	channel1 map[string]chan *events.Event
	events.UnimplementedEventServiceServer
	local  events.EventServiceClient
	logger logger.Logger
	mu     sync.Mutex
}

func (n *EventService) Register(server *grpc.Server) error {
	server.RegisterService(&events.EventService_ServiceDesc, n)
	// events.RegisterEventServiceServer(server, n)
	return nil
}

func (n *EventService) Get(ctx context.Context, req *events.GetEventRequest) (*events.GetEventResponse, error) {
	return n.local.Get(ctx, req)
}

func (n *EventService) Delete(ctx context.Context, req *events.DeleteEventRequest) (*events.DeleteEventResponse, error) {
	return n.local.Delete(ctx, req)
}

func (n *EventService) List(ctx context.Context, req *events.ListEventRequest) (*events.ListEventResponse, error) {
	return n.local.List(ctx, req)
}

func (s *EventService) Subscribe(req *events.SubscribeRequest, stream events.EventService_SubscribeServer) error {
	// Identify the client
	clientId := req.ClientId
	peer, _ := peer.FromContext(stream.Context())
	eventChan := make(chan *events.Event)

	s.logger.Info("client connected", "clientId", clientId, "address", peer.Addr.String())

	s.mu.Lock()
	// s.channel[clientId] = append(s.channel[clientId], eventChan)
	s.channel[clientId] = eventChan
	s.mu.Unlock()

	go func() {
		for {
			select {
			case n := <-eventChan:
				s.logger.Debug("got event from client", "eventType", n.Type, "objectId", n.GetObjectId(), "eventId", n.GetMeta().GetName(), "clientId", req.ClientId)
				err := stream.Send(n)
				if err != nil {
					s.logger.Error("unable to emit event to clients", "error", err, "eventType", n.Type, "objectId", n.GetObjectId(), "eventId", n.GetMeta().GetName(), "clientId", req.ClientId)
					return
				}
			case <-stream.Context().Done():
				s.logger.Info("client disconnected", "clientId", req.ClientId)
				s.mu.Lock()
				delete(s.channel, req.ClientId)
				s.mu.Unlock()
				return
			}
		}
	}()

	<-stream.Context().Done()
	return nil
}

func (s *EventService) Publish(ctx context.Context, req *events.PublishRequest) (*events.PublishResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.local.Publish(ctx, req)
	if err != nil {
		return nil, err
	}

	for client, ch := range s.channel {
		select {
		case ch <- req.Event:
			s.logger.Info("Notified client", "client", client)
		default:
			s.logger.Info("Client is too slow to receive events", "client", client)
		}
	}

	s.logger.Info("Successfullt published event", "event", req.GetEvent().GetType())
	return &events.PublishResponse{Event: req.GetEvent()}, nil
}

func NewService(repo repository.EventRepository, opts ...NewServiceOption) *EventService {
	s := &EventService{
		// channel: make(map[string][]chan *events.Event),
		channel: make(map[string]chan *events.Event),
		local: &local{
			repo: repo,
		},
		logger: logger.ConsoleLogger{},
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func NewEventFor(clientId, id string, t events.EventType) *events.Event {
	return &events.Event{
		Meta: &types.Meta{
			Name: uuid.New().String(),
		},
		ClientId: clientId,
		Type:     t,
		ObjectId: id,
	}
}
