package node

import (
	"context"
	"fmt"
	"sync"

	"github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/api/services/nodes/v1"
	"github.com/amimof/blipblop/pkg/repository"
	"github.com/amimof/blipblop/services"
	"github.com/amimof/blipblop/services/event"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
)

type local struct {
	repo        repository.NodeRepository
	mu          sync.Mutex
	eventClient *event.EventService
}

var _ nodes.NodeServiceClient = &local{}

func (l *local) Get(ctx context.Context, req *nodes.GetNodeRequest, _ ...grpc.CallOption) (*nodes.GetNodeResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	clientId := md.Get("blipblop_client_id")[0]
	node, err := l.Repo().Get(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	if node == nil {
		return nil, fmt.Errorf("node not found %s", req.GetId())
	}
	_, err = l.eventClient.Publish(ctx, &events.PublishRequest{Event: event.NewEventFor(clientId, req.GetId(), events.EventType_NodeGet)})
	if err != nil {
		return nil, err
	}
	return &nodes.GetNodeResponse{
		Node: node,
	}, nil
}

func (l *local) Create(ctx context.Context, req *nodes.CreateNodeRequest, _ ...grpc.CallOption) (*nodes.CreateNodeResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	clientId := md.Get("blipblop_client_id")[0]
	node := req.GetNode()
	if existing, _ := l.Repo().Get(ctx, node.GetMeta().GetName()); existing != nil {
		return nil, fmt.Errorf("node %s already exists", node.GetMeta().GetName())
	}

	err := services.EnsureMeta(node)
	if err != nil {
		return nil, err
	}

	// node.Created = timestamppb.New(time.Now())
	// node.Updated = timestamppb.New(time.Now())
	// node.Revision = 1

	err = l.Repo().Create(ctx, node)
	if err != nil {
		return nil, err
	}
	_, err = l.eventClient.Publish(ctx, &events.PublishRequest{Event: event.NewEventFor(clientId, req.GetNode().GetMeta().GetName(), events.EventType_NodeCreate)})
	if err != nil {
		return nil, err
	}
	return &nodes.CreateNodeResponse{
		Node: node,
	}, nil
}

func (l *local) Delete(ctx context.Context, req *nodes.DeleteNodeRequest, _ ...grpc.CallOption) (*nodes.DeleteNodeResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	clientId := md.Get("blipblop_client_id")[0]
	err := l.Repo().Delete(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	_, err = l.eventClient.Publish(ctx, &events.PublishRequest{Event: event.NewEventFor(clientId, req.GetId(), events.EventType_NodeDelete)})
	if err != nil {
		return nil, err
	}
	return &nodes.DeleteNodeResponse{
		Id: req.Id,
	}, nil
}

func (l *local) List(ctx context.Context, req *nodes.ListNodeRequest, _ ...grpc.CallOption) (*nodes.ListNodeResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	clientId := md.Get("blipblop_client_id")[0]
	nodeList, err := l.Repo().List(ctx)
	if err != nil {
		return nil, err
	}
	_, err = l.eventClient.Publish(ctx, &events.PublishRequest{Event: event.NewEventFor(clientId, "", events.EventType_NodeList)})
	if err != nil {
		return nil, err
	}

	return &nodes.ListNodeResponse{
		Nodes: nodeList,
	}, nil
}

func (l *local) Update(ctx context.Context, req *nodes.UpdateNodeRequest, _ ...grpc.CallOption) (*nodes.UpdateNodeResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	clientId := md.Get("blipblop_client_id")[0]
	updateMask := req.GetUpdateMask()
	updateNode := req.GetNode()
	existing, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}

	if updateMask != nil && updateMask.IsValid(existing) {
		proto.Merge(existing, updateNode)
	}

	err = l.Repo().Update(ctx, existing)
	if err != nil {
		return nil, err
	}
	_, err = l.eventClient.Publish(ctx, &events.PublishRequest{Event: event.NewEventFor(clientId, existing.GetMeta().GetName(), events.EventType_NodeUpdate)})
	if err != nil {
		return nil, err
	}
	return &nodes.UpdateNodeResponse{
		Node: existing,
	}, nil
}

func (l *local) Join(ctx context.Context, req *nodes.JoinRequest, _ ...grpc.CallOption) (*nodes.JoinResponse, error) {
	// clientId := ctx.Value("blipblop/client_id").(string)
	md, _ := metadata.FromIncomingContext(ctx)
	clientId := md.Get("blipblop_client_id")[0]

	if req.GetNode().GetMeta().GetName() == "" {
		return nil, fmt.Errorf("node name cannot be empty")
	}
	if node, _ := l.Get(ctx, &nodes.GetNodeRequest{Id: req.GetNode().GetMeta().GetName()}); node.GetNode() != nil {
		return &nodes.JoinResponse{
			Id: req.GetNode().GetMeta().GetName(),
		}, fmt.Errorf("Node %s already joined to cluster", req.GetNode().GetMeta().GetName())
	}
	_, err := l.Create(ctx, &nodes.CreateNodeRequest{Node: req.Node})
	if err != nil {
		return nil, err
	}
	_, err = l.eventClient.Publish(ctx, &events.PublishRequest{Event: event.NewEventFor(clientId, req.GetNode().GetMeta().GetName(), events.EventType_NodeJoin)})
	if err != nil {
		return nil, err
	}
	return &nodes.JoinResponse{
		Id: req.GetNode().GetMeta().GetName(),
	}, nil
}

func (l *local) Forget(ctx context.Context, req *nodes.ForgetRequest, _ ...grpc.CallOption) (*nodes.ForgetResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	clientId := md.Get("blipblop_client_id")[0]
	_, err := l.Delete(ctx, &nodes.DeleteNodeRequest{Id: req.GetId()})
	if err != nil {
		return nil, err
	}
	_, err = l.eventClient.Publish(ctx, &events.PublishRequest{Event: event.NewEventFor(clientId, req.GetId(), events.EventType_NodeForget)})
	if err != nil {
		return nil, err
	}
	return &nodes.ForgetResponse{
		Id: req.Id,
	}, nil
}

func (l *local) Repo() repository.NodeRepository {
	if l.repo != nil {
		return l.repo
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return repository.NewNodeInMemRepo()
}
