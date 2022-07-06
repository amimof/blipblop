package services

import (
	"context"
	"errors"
	"fmt"
	"github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/api/services/nodes/v1"
	"github.com/amimof/blipblop/internal/models"
	"github.com/amimof/blipblop/internal/repo"
	"github.com/amimof/blipblop/pkg/util"
	"sync"
	"time"
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
			//Name: "ContainerCreate",
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
	if node, _ := n.Get(req.Node.Name); node != nil {
		return &nodes.JoinResponse{
			Id: req.Node.Name,
		}, errors.New(fmt.Sprintf("Node %s already joined to cluster", req.Node.Name))
	}
	node := &models.Node{
		Metadata: models.Metadata{
			Name:     util.PtrString(req.Node.Name),
			Created:  time.Now(),
			Updated:  time.Now(),
			Revision: 1,
		},
		Status: &models.NodeStatus{
			IPs:      req.Node.Status.Ips,
			HostName: req.Node.Status.Hostname,
			Arch:     req.Node.Status.Arch,
			Os:       req.Node.Status.Os,
		},
	}
	err := n.Create(node)
	if err != nil {
		return nil, err
	}
	return &nodes.JoinResponse{
		Id: req.Node.Name,
	}, nil
}

func (n *NodeService) Forget(ctx context.Context, req *nodes.ForgetRequest) (*nodes.ForgetResponse, error) {
	err := n.Delete(req.Id)
	if err != nil {
		return nil, err
	}
	return &nodes.ForgetResponse{
		Id: req.Id,
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
