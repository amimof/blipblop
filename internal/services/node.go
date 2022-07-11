package services

import (
	"context"
	"errors"
	"fmt"
	"github.com/amimof/blipblop/api/services/nodes/v1"
	"github.com/amimof/blipblop/internal/repo"
	"google.golang.org/protobuf/proto"
	"sync"
)

var nodeService *NodeService

type NodeService struct {
	mu sync.Mutex
	nodes.UnimplementedNodeServiceServer
	repo repo.NodeRepo
}

func (n *NodeService) Get(ctx context.Context, req *nodes.GetNodeRequest) (*nodes.GetNodeResponse, error) {
	node, err := n.Repo().Get(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &nodes.GetNodeResponse{
		Node: node,
	}, nil
}

func (n *NodeService) Create(ctx context.Context, req *nodes.CreateNodeRequest) (*nodes.CreateNodeResponse, error) {
	err := n.Repo().Create(ctx, req.Node)
	if err != nil {
		return nil, err
	}
	node, err := n.Repo().Get(ctx, req.Node.Name)
	return &nodes.CreateNodeResponse{
		Node: node,
	}, nil
}

func (n *NodeService) Delete(ctx context.Context, req *nodes.DeleteNodeRequest) (*nodes.DeleteNodeResponse, error) {
	err := n.Repo().Delete(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &nodes.DeleteNodeResponse{
		Id: req.Id,
	}, nil
}

func (n *NodeService) List(ctx context.Context, req *nodes.ListNodeRequest) (*nodes.ListNodeResponse, error) {
	res, err := n.Repo().List(ctx)
	if err != nil {
		return nil, err
	}
	return &nodes.ListNodeResponse{
		Nodes: res,
	}, nil
}

func (n *NodeService) Update(ctx context.Context, req *nodes.UpdateNodeRequest) (*nodes.UpdateNodeResponse, error) {
	updateMask := req.GetUpdateMask()
	updateNode := req.GetNode()
	existingNode, err := n.Repo().Get(ctx, updateNode.GetName())
	if err != nil {
		return nil, err
	}
	if updateMask != nil && updateMask.IsValid(existingNode) {
		proto.Merge(existingNode, updateNode)
	}
	err = n.Repo().Update(ctx, existingNode)
	if err != nil {
		return nil, err
	}
	return &nodes.UpdateNodeResponse{
		Node: req.Node,
	}, nil
}

func (n *NodeService) Join(ctx context.Context, req *nodes.JoinRequest) (*nodes.JoinResponse, error) {
	if node, _ := n.Get(ctx, &nodes.GetNodeRequest{Id: req.Node.Name}); node.GetNode() != nil {
		return &nodes.JoinResponse{
			Id: req.Node.Name,
		}, errors.New(fmt.Sprintf("Node %s already joined to cluster", req.Node.Name))
	}
	_, err := n.Create(ctx, &nodes.CreateNodeRequest{Node: req.Node})
	if err != nil {
		return nil, err
	}
	return &nodes.JoinResponse{
		Id: req.Node.Name,
	}, nil
}

func (n *NodeService) Forget(ctx context.Context, req *nodes.ForgetRequest) (*nodes.ForgetResponse, error) {
	_, err := n.Delete(ctx, &nodes.DeleteNodeRequest{Id: req.Id})
	if err != nil {
		return nil, err
	}
	return &nodes.ForgetResponse{
		Id: req.Id,
	}, nil
}

func (n *NodeService) Repo() repo.NodeRepo {
	if n.repo != nil {
		return n.repo
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	return repo.NewNodeRepo()
}

func newNodeService(r repo.NodeRepo) *NodeService {
	return &NodeService{
		repo: r,
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
