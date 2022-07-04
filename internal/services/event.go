package services

import (
	"context"
	"github.com/amimof/blipblop/api/services/events/v1"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
	"io"
	"log"
)

var eventService *EventService

type EventService struct {
	channel map[string][]chan *events.Event
	events.UnimplementedEventServiceServer
}

func (n *EventService) Subscribe(req *events.SubscribeRequest, stream events.EventService_SubscribeServer) error {
	log.Printf("Added subscriber %s", req.Id)
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

func (n *EventService) Publish(ctx context.Context, req *events.PublishRequest) (*emptypb.Empty, error) {
	for k, _ := range n.channel {
		for _, ch := range n.channel[k] {
			ch <- req.Event
		}
	}
	return nil, nil
}

func (n *EventService) FireEvent(stream events.EventService_FireEventServer) error {
	ev, err := stream.Recv()
	if err == io.EOF {
		log.Println("Got EOF while reading from stream")
		return nil
	}
	if err != nil {
		return err
	}

	ack := events.EventAck{Status: "SENT"}
	stream.SendAndClose(&ack)

	go func() {
		streams := n.channel["asd"]
		for _, evChan := range streams {
			evChan <- ev
		}
	}()

	return nil
}

func newEventService() *EventService {
	return &EventService{
		channel: make(map[string][]chan *events.Event),
	}
}

func NewEventService() *EventService {
	if eventService == nil {
		eventService = newEventService()
	}
	return eventService
}
