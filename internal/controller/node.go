package controller

import (
	"context"
	"github.com/amimof/blipblop/internal/models"
	"github.com/amimof/blipblop/internal/repo"
	"sync"
)

var nodeController *NodeController

type NodeController struct {
	mu sync.Mutex
	repo    repo.NodeRepo
	nodes   []*models.Node
}

func (n *NodeController) Get(id string) (*models.Node, error) {
	node, err := n.Repo().Get(context.Background(), id)
	if err != nil {
		return nil, err
	}
	return node, err
}

func (n *NodeController) All() ([]*models.Node, error) {
	return n.Repo().GetAll(context.Background())
}

func (n *NodeController) Create(node *models.Node) error {
	return n.Repo().Create(context.Background(), node)
}

func (n *NodeController) Delete(id string) error {
	return n.Repo().Delete(context.Background(), id)
}

func (n *NodeController) Repo() repo.NodeRepo {
	if n.repo != nil {
		return n.repo
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	return repo.NewNodeRepo()
}

func newNodeController(r repo.NodeRepo) *NodeController {
	return &NodeController{
		repo:    r,
	}
}

func NewNodeController() *NodeController {
	if nodeController == nil {
		nodeController = newNodeController(repo.NewNodeRepo())
	}
	return nodeController
}

func NewNodeControllerWithRepo(r repo.NodeRepo) *NodeController {
	n := NewNodeController()
	n.repo = r
	return n
}
