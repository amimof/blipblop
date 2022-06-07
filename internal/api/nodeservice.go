package api

import (
	"github.com/amimof/blipblop/pkg/util"
	"github.com/amimof/blipblop/internal/models"
	"github.com/amimof/blipblop/internal/repo"
	"github.com/amimof/blipblop/internal/services"
	"github.com/gogo/protobuf/types"
	proto "github.com/amimof/blipblop/proto"
	"errors"
	"context"
	"fmt"
	"io"
	"log"
	"time"
)

type nodeServiceServer struct {
	proto.UnimplementedNodeServiceServer
	channel map[string][]chan *proto.Event
}

func (n *nodeServiceServer) Join(ctx context.Context, req *proto.JoinRequest) (*proto.JoinResponse, error) {
	svc := services.NewNodeService(repo.NewNodeRepo())
	if node, _ := svc.Get(req.Node.Id); node != nil {
		return &proto.JoinResponse{
			Node: req.Node,
			//At: types.TimestampNow(),
			Status: proto.Status_JoinFail,
		}, errors.New(fmt.Sprintf("Node %s already joined to cluster", req.Node.Id))
	}
	node := &models.Node{
		Name: util.PtrString(req.Node.Id),
		Created: types.TimestampNow().String(),
	}
	err := svc.Create(node)
	if err != nil {
		return nil, err
	}
	return &proto.JoinResponse{
		Node: req.Node,
		//At: types.TimestampNow(),
		Status: proto.Status_JoinSuccess,
	}, nil
}

func (n *nodeServiceServer) Subscribe(node *proto.Node, stream proto.NodeService_SubscribeServer) error {
	eventChan := make(chan *proto.Event)
	n.channel[node.Id] = append(n.channel[node.Id], eventChan)

	go func() {
		for {
			unit := &models.Unit{
				Name:  util.PtrString("prometheus-deployment"),
				Image: util.PtrString("quay.io/prometheus/prometheus:latest"),
			}
			d, err := unit.Encode()
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
				Payload: d,
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
