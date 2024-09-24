package event

import (
	"context"
	"errors"
	"log"

	"github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/api/types/v1"
	"github.com/amimof/blipblop/pkg/logger"
	"github.com/amimof/blipblop/pkg/repository"
	"github.com/google/uuid"
	"google.golang.org/grpc"
)

var ErrClientExists = errors.New("client already exists")

type NewServiceOption func(s *EventService)

func WithLogger(l logger.Logger) NewServiceOption {
	return func(s *EventService) {
		s.logger = l
	}
}

type EventService struct {
	channel map[string][]chan *events.Event
	events.UnimplementedEventServiceServer
	local  events.EventServiceClient
	logger logger.Logger
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

func (n *EventService) Subscribe(req *events.SubscribeRequest, stream events.EventService_SubscribeServer) error {
	eventChan := make(chan *events.Event)
	if _, ok := n.channel[req.ClientId]; ok {
		return ErrClientExists
	}
	n.channel[req.ClientId] = append(n.channel[req.ClientId], eventChan)
	log.Printf("Client %s joined", req.ClientId)
	for {
		select {
		case <-stream.Context().Done():
			log.Printf("Client %s left", req.ClientId)
			delete(n.channel, req.ClientId)
			return nil
		case n := <-eventChan:
			log.Printf("Got event %s (%s) from client %s", n.Type, n.GetMeta().GetName(), req.ClientId)
			err := stream.Send(n)
			if err != nil {
				log.Printf("Unable to emit event to clients: %s", err.Error())
			}
		}
	}
}

func (n *EventService) Publish(ctx context.Context, req *events.PublishRequest) (*events.PublishResponse, error) {
	_, err := n.local.Publish(ctx, req)
	if err != nil {
		return nil, err
	}
	for k := range n.channel {
		for _, ch := range n.channel[k] {
			ch <- req.Event
		}
	}
	return &events.PublishResponse{Event: req.GetEvent()}, nil
}

func NewService(repo repository.EventRepository, opts ...NewServiceOption) *EventService {
	s := &EventService{
		channel: make(map[string][]chan *events.Event),
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

func NewEventFor(id string, t events.EventType) *events.Event {
	return &events.Event{
		Meta: &types.Meta{
			Name: uuid.New().String(),
		},
		Type:     t,
		ObjectId: id,
	}
}
