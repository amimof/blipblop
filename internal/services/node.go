package services

import (
	"context"
	"log"
	"errors"
	"fmt"
	"github.com/amimof/blipblop/internal/models"
	"github.com/amimof/blipblop/internal/repo"
	"github.com/amimof/blipblop/pkg/util"
	"github.com/amimof/blipblop/api/services/nodes/v1"
	"github.com/amimof/blipblop/api/services/events/v1"
	"github.com/golang/protobuf/ptypes"
	"sync"
)

var nodeService *NodeService

type NodeService struct {
	mu sync.Mutex
	nodes.UnimplementedNodeServiceServer
	repo    repo.NodeRepo
	nodes   []*models.Node
	channel map[string][]chan *events.Event
}

func (n *NodeService) Get(id string) (*models.Node, error) {
	node, err := n.Repo().Get(context.Background(), id)
	if err != nil {
		return nil, err
	}
	for _, ch := range n.channel[id] {
		ch <- &events.Event{
			Name: "ContainerCreate",
			Type: events.EventType_ContainerCreate,
			// Node: &nodes.Node{
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

func (n *NodeService) Join(ctx context.Context, req *nodes.JoinRequest) (*nodes.JoinResponse, error) {
	if node, _ := n.Get(req.Node.Id); node != nil {
		return &nodes.JoinResponse{
			Node: req.Node,
			//At: types.TimestampNow(),
			Status: nodes.Status_JoinFail,
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
	return &nodes.JoinResponse{
		Node: req.Node,
		//At: types.TimestampNow(),
		Status: nodes.Status_JoinSuccess,
	}, nil
}

func newNodeService(r repo.NodeRepo) *NodeService {
	return &NodeService{
		repo:    r,
		channel: make(map[string][]chan *events.Event),
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
