package node

import (
	"context"

	"github.com/amimof/blipblop/api/services/nodes/v1"
	"github.com/amimof/blipblop/pkg/repository"
	"github.com/amimof/blipblop/services/event"
	"google.golang.org/grpc"
)

type NodeService struct {
	nodes.UnimplementedNodeServiceServer
	local nodes.NodeServiceClient
}

func (n *NodeService) Register(server *grpc.Server) error {
	nodes.RegisterNodeServiceServer(server, n)
	return nil
}

func (n *NodeService) Get(ctx context.Context, req *nodes.GetNodeRequest) (*nodes.GetNodeResponse, error) {
	return n.local.Get(ctx, req)
}

func (n *NodeService) Create(ctx context.Context, req *nodes.CreateNodeRequest) (*nodes.CreateNodeResponse, error) {
	return n.local.Create(ctx, req)
}

func (n *NodeService) Delete(ctx context.Context, req *nodes.DeleteNodeRequest) (*nodes.DeleteNodeResponse, error) {
	return n.local.Delete(ctx, req)
}

func (n *NodeService) List(ctx context.Context, req *nodes.ListNodeRequest) (*nodes.ListNodeResponse, error) {
	return n.local.List(ctx, req)
}

func (n *NodeService) Update(ctx context.Context, req *nodes.UpdateNodeRequest) (*nodes.UpdateNodeResponse, error) {
	return n.local.Update(ctx, req)
}

func (n *NodeService) Join(ctx context.Context, req *nodes.JoinRequest) (*nodes.JoinResponse, error) {
	return n.local.Join(ctx, req)
}

func (n *NodeService) Forget(ctx context.Context, req *nodes.ForgetRequest) (*nodes.ForgetResponse, error) {
	return n.local.Forget(ctx, req)
}

func NewService(repo repository.Repository, ev *event.EventService) *NodeService {
	return &NodeService{
		local: &local{
			repo:        repo,
			eventClient: ev,
		},
	}
}
