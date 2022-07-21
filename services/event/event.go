package event

import (
	"context"
	"github.com/amimof/blipblop/api/services/events/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
	"log"
	"time"
)

type EventService struct {
	channel map[string][]chan *events.Event
	events.UnimplementedEventServiceServer
	local events.EventServiceClient
}

func (n *EventService) Register(server *grpc.Server) error {
	events.RegisterEventServiceServer(server, n)
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
	n.channel[req.Id] = append(n.channel[req.Id], eventChan)
	log.Printf("Node %s joined", req.Id)
	for {
		select {
		case <-stream.Context().Done():
			log.Printf("Node %s left", req.Id)
			delete(n.channel, req.Id)
			return nil
		case n := <-eventChan:
			log.Printf("Got event %s (%s) from client %s", n.Type, n.Id, req.Id)
			stream.Send(n)
		}
	}
}

func (n *EventService) Publish(ctx context.Context, req *events.PublishRequest) (*events.PublishResponse, error) {
	_, err := n.local.Publish(ctx, req)
	if err != nil {
		return nil, err
	}
	for k, _ := range n.channel {
		for _, ch := range n.channel[k] {
			ch <- req.Event
		}
	}
	return &events.PublishResponse{Event: req.GetEvent()}, nil
}

func NewService(repo Repo) *EventService {
	return &EventService{
		channel: make(map[string][]chan *events.Event),
		local: &local{
			repo: repo,
		},
	}
}

func NewEventFor(id string, t events.EventType) *events.Event {
	return &events.Event{
		Type:      t,
		Id:        id,
		EventId:   uuid.New().String(),
		Timestamp: timestamppb.New(time.Now()),
	}
}
