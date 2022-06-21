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
			// Node: &proto.Node{
			// 	Id: id,
			// },
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
