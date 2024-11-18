package event

import (
	"context"
	"errors"

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
	logger   logger.Logger
	exchange *events.Exchange
}

func (n *EventService) Register(server *grpc.Server) error {
	server.RegisterService(&eventsv1.EventService_ServiceDesc, n)
	return nil
}

func (s *EventService) Subscribe(req *eventsv1.SubscribeRequest, stream eventsv1.EventService_SubscribeServer) error {
	return s.exchange.Forward(req, stream)
}

func (s *EventService) Publish(ctx context.Context, req *eventsv1.PublishRequest) (*eventsv1.PublishResponse, error) {
	err := s.exchange.Publish(ctx, req)
	return &eventsv1.PublishResponse{Event: req.GetEvent()}, err
}

func NewService(repo repository.EventRepository, opts ...NewServiceOption) *EventService {
	s := &EventService{
		logger: logger.ConsoleLogger{},
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}
