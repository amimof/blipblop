package services

import (
	"context"
	"errors"
	"fmt"
	"github.com/amimof/blipblop/internal/models"
	"github.com/amimof/blipblop/internal/repo"
	"github.com/amimof/blipblop/pkg/util"
	proto "github.com/amimof/blipblop/proto"
	"github.com/golang/protobuf/ptypes"
	"io"
	"log"
	"sync"
)

var nodeService *NodeService

type NodeService struct {
	mu sync.Mutex
	proto.UnimplementedNodeServiceServer
	repo    repo.NodeRepo
	nodes   []*models.Node
	channel map[string][]chan *proto.Event
}

func (n *NodeService) Get(id string) (*models.Node, error) {
	node, err := n.Repo().Get(context.Background(), id)
	if err != nil {
		return nil, err
	}
	for _, ch := range n.channel[id] {
		ch <- &proto.Event{
			Name: "ContainerCreate",
			Type: proto.EventType_ContainerCreate,
			Node: &proto.Node{
				Id: id,
			},
		}
	}
	return node, err
}

func (n *NodeService) All() ([]*models.Node, error) {
	return n.Repo().GetAll(context.Background())
}

func (n *NodeService) Create(node *models.Node) error {
	return n.Repo().Create(context.Background(), node)
}

func (n *NodeService) Delete(id string) error {
	return n.Repo().Delete(context.Background(), id)
}

func (n *NodeService) Repo() repo.NodeRepo {
	if n.repo != nil {
		return n.repo
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	return repo.NewNodeRepo()
}

func (n *NodeService) Join(ctx context.Context, req *proto.JoinRequest) (*proto.JoinResponse, error) {
	if node, _ := n.Get(req.Node.Id); node != nil {
		return &proto.JoinResponse{
			Node: req.Node,
			//At: types.TimestampNow(),
			Status: proto.Status_JoinFail,
		}, errors.New(fmt.Sprintf("Node %s already joined to cluster", req.Node.Id))
	}
	node := &models.Node{
		Name:    util.PtrString(req.Node.Id),
		Created: ptypes.TimestampNow().String(),
	}
	err := n.Create(node)
	if err != nil {
		return nil, err
	}
	return &proto.JoinResponse{
		Node: req.Node,
		//At: types.TimestampNow(),
		Status: proto.Status_JoinSuccess,
	}, nil
}

func (n *NodeService) Subscribe(node *proto.Node, stream proto.NodeService_SubscribeServer) error {
	eventChan := make(chan *proto.Event)
	n.channel[node.Id] = append(n.channel[node.Id], eventChan)

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

func (n *NodeService) FireEvent(stream proto.NodeService_FireEventServer) error {
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

func newNodeService(r repo.NodeRepo) *NodeService {
	return &NodeService{
		repo:    r,
		channel: make(map[string][]chan *proto.Event),
	}
}

func NewNodeService() *NodeService {
	if nodeService == nil {
		nodeService = newNodeService(repo.NewNodeRepo())
	}
	return nodeService
}

func NewNodeServiceWithRepo(r repo.NodeRepo) *NodeService {
	n := NewNodeService()
	n.repo = r
	return n
}
