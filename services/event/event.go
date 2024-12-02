package event

import (
	"context"
	"errors"
	"fmt"

	eventsv1 "github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/pkg/events"
	"github.com/amimof/blipblop/pkg/logger"
	"github.com/amimof/blipblop/pkg/repository"
	"google.golang.org/grpc"
)

var ErrClientExists = errors.New("client already exists")

type NewServiceOption func(s *EventService)

func WithLogger(l logger.Logger) NewServiceOption {
	return func(s *EventService) {
		s.logger = l
	}
}

func WithExchange(e *events.Exchange) NewServiceOption {
	return func(s *EventService) {
		s.exchange = e
	}
}

type EventService struct {
	eventsv1.UnimplementedEventServiceServer
	local    eventsv1.EventServiceClient
	logger   logger.Logger
	exchange *events.Exchange
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
	return s.exchange.Forward(req, stream)
}

func (s *EventService) Publish(ctx context.Context, req *eventsv1.PublishRequest) (*eventsv1.PublishResponse, error) {
	fmt.Println("PUBLISHING")
	err := s.exchange.Publish(ctx, req)
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
