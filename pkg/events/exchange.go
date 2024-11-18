package events

import (
	"context"
	"sync"

	eventsv1 "github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/pkg/logger"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

type NewExchangeOption func(*Exchange)

func WithLogger(l logger.Logger) NewExchangeOption {
	return func(e *Exchange) {
		e.logger = l
	}
}

type Exchange struct {
	subscribers map[string][]chan *eventsv1.Event
	mu          sync.Mutex
	logger      logger.Logger
}

func (s *Exchange) Forward(req *eventsv1.SubscribeRequest, stream eventsv1.EventService_SubscribeServer) error {
	// Identify the client
	ctx := stream.Context()
	clientId := req.ClientId
	peer, _ := peer.FromContext(ctx)
	eventChan := make(chan *eventsv1.Event)

	s.logger.Debug("client connected", "clientId", clientId, "address", peer.Addr.String())

	s.mu.Lock()
	s.subscribers[clientId] = append(s.subscribers[clientId], eventChan)
	s.mu.Unlock()
	go func() {
		for {
			select {
			case n := <-eventChan:

				s.logger.Debug("forwarding event from client", "eventType", n.Type, "objectId", n.GetObjectId(), "eventId", n.GetMeta().GetName(), "clientId", req.ClientId)
				err := stream.Send(n)
				if err != nil {
					s.logger.Error("unable to emit event to clients", "error", err, "eventType", n.Type, "objectId", n.GetObjectId(), "eventId", n.GetMeta().GetName(), "clientId", req.ClientId)
					return
				}
			case <-ctx.Done():
				s.logger.Debug("client disconnected", "clientId", req.ClientId)
				s.mu.Lock()
				delete(s.subscribers, req.ClientId)
				s.mu.Unlock()

				// Get node name from context
				if md, ok := metadata.FromIncomingContext(ctx); ok {
					if nodeName, ok := md["blipblop_node_name"]; ok && len(nodeName) > 0 {
						err := s.Publish(ctx, &eventsv1.PublishRequest{Event: &eventsv1.Event{ObjectId: nodeName[0], Type: eventsv1.EventType_NodeForget}})
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

func (s *Exchange) Publish(ctx context.Context, req *eventsv1.PublishRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for client, sub := range s.subscribers {
		for _, ch := range sub {
			select {
			case ch <- req.Event:
				s.logger.Debug("published event to client", "client", client, "event", req.GetEvent().GetType().String())
			default:
				s.logger.Debug("client is too slow to receive events", "client", client)
			}
		}
	}

	return nil
}

func (s *Exchange) Subscribe(ctx context.Context) (<-chan *eventsv1.Event, <-chan error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	eventChan := make(chan *eventsv1.Event, 10)

	clientId := "NOID"
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		clientIdMd := md.Get("blipblop_client_id")
		if len(clientIdMd) > 0 {
			clientId = clientIdMd[0]
		}
	}

	s.subscribers[clientId] = append(s.subscribers[clientId], eventChan)

	return eventChan, nil
}

func (e *Exchange) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, ch := range e.subscribers {
		for _, sub := range ch {
			close(sub)
		}
	}
}

func NewExchange(opts ...NewExchangeOption) *Exchange {
	e := &Exchange{
		subscribers: make(map[string][]chan *eventsv1.Event),
		logger:      logger.ConsoleLogger{},
	}

	for _, opt := range opts {
		opt(e)
	}

	return e
}
