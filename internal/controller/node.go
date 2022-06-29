package controller

import (
	"context"
	"github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/internal/models"
	"github.com/amimof/blipblop/internal/repo"
	"github.com/amimof/blipblop/pkg/client"
	"sync"
)

var nodeController *NodeController

type NodeController struct {
	mu sync.Mutex
	repo    repo.NodeRepo
	nodes   []*models.Node
	client *client.LocalClient
}

func (n *NodeController) Get(id string) (*models.Node, error) {
	ctx := context.Background()
	node, err := n.Repo().Get(ctx, id)
	if err != nil {
		return nil, err
	}
	e := &events.Event{
		Name: "GetNode",
		Type: events.EventType_ContainerCreate,
	}
	_, err = n.client.EventService().Publish(ctx, &events.PublishRequest{Id: id, Event: e})
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

func newNodeController(client *client.LocalClient, r repo.NodeRepo) *NodeController {
	return &NodeController{
		repo:    r,
		client: client,
	}
}

func NewNodeController(client *client.LocalClient) *NodeController {
	if nodeController == nil {
		nodeController = newNodeController(client, repo.NewNodeRepo())
	}
	return nodeController
}

func NewNodeControllerWithRepo(client *client.LocalClient, r repo.NodeRepo) *NodeController {
	n := NewNodeController(client)
	n.repo = r
	return n
}
