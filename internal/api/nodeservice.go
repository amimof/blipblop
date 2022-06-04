package api

import (
	"bytes"
	"encoding/gob"
	"github.com/amimof/blipblop/internal/models"
	proto "github.com/amimof/blipblop/proto"
	"io"
	"log"
	"time"
)

type nodeServiceServer struct {
	proto.UnimplementedNodeServiceServer
	channel map[string][]chan *proto.Event
}

func (n *nodeServiceServer) JoinNode(node *proto.Node, stream proto.NodeService_JoinNodeServer) error {
	eventChan := make(chan *proto.Event)
	n.channel[node.Id] = append(n.channel[node.Id], eventChan)

	go func() {
		for {
			//log.Printf("I have %d nodes", len(n.channel))
			name := "prometheus-deployment"
			image := "quay.io/prometheus/prometheus:latest"
			unit := &models.Unit{
				Name:  &name,
				Image: &image,
			}
			var buf bytes.Buffer
			enc := gob.NewEncoder(&buf)
			err := enc.Encode(unit)
			if err != nil {
				log.Printf("Error encoding: %s", err.Error())
				continue
			}
			e := &proto.Event{
				Name: "ContainerCreate",
				Type: proto.EventType_ContainerCreate,
				Node: &proto.Node{
					Id: "asdasd",
				},
				Payload: buf.Bytes(),
			}
			eventChan <- e
			time.Sleep(time.Second * 2)
		}
	}()

	log.Printf("Node %s joined", node.Id)

	for {
		select {
		case <-stream.Context().Done():
			log.Printf("Node %s left", node.Id)
			delete(n.channel, node.Id)
			return nil
		case n := <-eventChan:
			log.Printf("Got event %s from node %s", n.Name, n.Node.Id)
			stream.Send(n)
		}
	}
}

func (n *nodeServiceServer) FireEvent(stream proto.NodeService_FireEventServer) error {
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
		streams := n.channel[ev.Node.Id]
		for _, evChan := range streams {
			evChan <- ev
		}
	}()

	return nil
}
