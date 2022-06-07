package services

import (
	"context"
	"github.com/amimof/blipblop/internal/models"
	"github.com/amimof/blipblop/internal/repo"
)

var nodeService *NodeService

type NodeService struct {
	repo  repo.NodeRepo
	nodes []*models.Node
}

func (u *NodeService) Get(id string) (*models.Node, error) {
	return u.repo.Get(context.Background(), id)
}

func (u *NodeService) All() ([]*models.Node, error) {
	return u.repo.GetAll(context.Background())
}

func (u *NodeService) Create(node *models.Node) error {
	return u.repo.Create(context.Background(), node)
}

func (u *NodeService) Delete(id string) error {
	return u.repo.Delete(context.Background(), id)
}

func newNodeService(repo repo.NodeRepo) *NodeService {
	return &NodeService{
		repo: repo,
	}
}

func NewNodeService(repo repo.NodeRepo) *NodeService {
	if nodeService == nil {
		nodeService = newNodeService(repo)
	}
	return nodeService
} 
