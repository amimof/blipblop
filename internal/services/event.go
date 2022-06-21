package services

import (
	proto "github.com/amimof/blipblop/proto"
	"io"
	"log"
)

var eventService *EventService

type EventService struct {
	channel map[string][]chan *proto.Event
	proto.UnimplementedEventServiceServer
}

func (n *EventService) Subscribe(req *proto.SubscribeRequest, stream proto.EventService_SubscribeServer) error {
	eventChan := make(chan *proto.Event)
	n.channel[req.Id] = append(n.channel[req.Id], eventChan)

	// go func() {
	// 	for {
	// 		unit := &models.Unit{
	// 			Name:  util.PtrString("prometheus-deployment"),
	// 			Image: util.PtrString("quay.io/prometheus/prometheus:latest"),
	// 		}
	// 		d, err := unit.Encode()
	// 		if err != nil {
	// 			log.Printf("Error encoding: %s", err.Error())
	// 			continue
	// 		}
	// 		e := &proto.Event{
	// 			Name: "ContainerCreate",
	// 			Type: proto.EventType_ContainerCreate,
	// 			Node: &proto.Node{
	// 				Id: "asdasd",
	// 			},
	// 			Payload: d,
	// 		}
	// 		eventChan <- e
	// 		time.Sleep(time.Second * 2)
	// 	}
	// }()

	log.Printf("Node %s joined", req.Id)

	for {
		select {
		case <-stream.Context().Done():
			log.Printf("Node %s left", req.Id)
			delete(n.channel, req.Id)
			return nil
		case n := <-eventChan:
			log.Printf("Got event %s from client %s", n.Name, req.Id)
			stream.Send(n)
		}
	}
}

func (n *EventService) FireEvent(stream proto.EventService_FireEventServer) error {
	ev, err := stream.Recv()
	if err == io.EOF {
		log.Println("Got EOF while reading from stream")
		return nil
	}
	if err != nil {
		return err
	}

	ack := proto.EventAck{Status: "SENT"}
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
		channel: make(map[string][]chan *proto.Event),
	}
}

func NewEventService() *EventService {
	if eventService == nil {
		eventService = newEventService()
	}
	return eventService
}
