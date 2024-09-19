package node

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/api/services/nodes/v1"
	"github.com/amimof/blipblop/pkg/repository"
	"github.com/amimof/blipblop/services/event"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type local struct {
	repo        repository.NodeRepository
	mu          sync.Mutex
	eventClient *event.EventService
}

var _ nodes.NodeServiceClient = &local{}

func (l *local) Get(ctx context.Context, req *nodes.GetNodeRequest, _ ...grpc.CallOption) (*nodes.GetNodeResponse, error) {
	node, err := l.Repo().Get(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	if node == nil {
		return nil, fmt.Errorf("node not found %s", req.GetId())
	}
	_, err = l.eventClient.Publish(ctx, &events.PublishRequest{Event: event.NewEventFor(req.GetId(), events.EventType_NodeGet)})
	if err != nil {
		return nil, err
	}
	return &nodes.GetNodeResponse{
		Node: node,
	}, nil
}

func (l *local) Create(ctx context.Context, req *nodes.CreateNodeRequest, _ ...grpc.CallOption) (*nodes.CreateNodeResponse, error) {
	node := req.GetNode()
	if existing, _ := l.Repo().Get(ctx, node.GetName()); existing != nil {
		return nil, fmt.Errorf("node %s already exists", node.GetName())
	}

	node.Created = timestamppb.New(time.Now())
	node.Updated = timestamppb.New(time.Now())
	node.Revision = 1

	err := l.Repo().Create(ctx, node)
	if err != nil {
		return nil, err
	}
	_, err = l.eventClient.Publish(ctx, &events.PublishRequest{Event: event.NewEventFor(req.GetNode().GetName(), events.EventType_NodeCreate)})
	if err != nil {
		return nil, err
	}
	return &nodes.CreateNodeResponse{
		Node: node,
	}, nil
}

func (l *local) Delete(ctx context.Context, req *nodes.DeleteNodeRequest, _ ...grpc.CallOption) (*nodes.DeleteNodeResponse, error) {
	err := l.Repo().Delete(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	_, err = l.eventClient.Publish(ctx, &events.PublishRequest{Event: event.NewEventFor(req.GetId(), events.EventType_NodeDelete)})
	if err != nil {
		return nil, err
	}
	return &nodes.DeleteNodeResponse{
		Id: req.Id,
	}, nil
}

func (l *local) List(ctx context.Context, req *nodes.ListNodeRequest, _ ...grpc.CallOption) (*nodes.ListNodeResponse, error) {
	nodeList, err := l.Repo().List(ctx)
	if err != nil {
		return nil, err
	}
	_, err = l.eventClient.Publish(ctx, &events.PublishRequest{Event: event.NewEventFor("", events.EventType_NodeList)})
	if err != nil {
		return nil, err
	}

	return &nodes.ListNodeResponse{
		Nodes: nodeList,
	}, nil
}

func (l *local) Update(ctx context.Context, req *nodes.UpdateNodeRequest, _ ...grpc.CallOption) (*nodes.UpdateNodeResponse, error) {
	updateMask := req.GetUpdateMask()
	updateNode := req.GetNode()
	existing, err := l.Repo().Get(ctx, updateNode.GetName())
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
	_, err = l.eventClient.Publish(ctx, &events.PublishRequest{Event: event.NewEventFor(updateNode.GetName(), events.EventType_NodeUpdate)})
	if err != nil {
		return nil, err
	}
	return &nodes.UpdateNodeResponse{
		Node: existing,
	}, nil
}

func (l *local) Join(ctx context.Context, req *nodes.JoinRequest, _ ...grpc.CallOption) (*nodes.JoinResponse, error) {
	if node, _ := l.Get(ctx, &nodes.GetNodeRequest{Id: req.GetNode().GetName()}); node.GetNode() != nil {
		return &nodes.JoinResponse{
			Id: req.GetNode().GetName(),
		}, fmt.Errorf("Node %s already joined to cluster", req.Node.Name)
	}
	_, err := l.Create(ctx, &nodes.CreateNodeRequest{Node: req.Node})
	if err != nil {
		return nil, err
	}
	_, err = l.eventClient.Publish(ctx, &events.PublishRequest{Event: event.NewEventFor(req.GetNode().GetName(), events.EventType_NodeJoin)})
	if err != nil {
		return nil, err
	}
	return &nodes.JoinResponse{
		Id: req.GetNode().GetName(),
	}, nil
}

func (l *local) Forget(ctx context.Context, req *nodes.ForgetRequest, _ ...grpc.CallOption) (*nodes.ForgetResponse, error) {
	_, err := l.Delete(ctx, &nodes.DeleteNodeRequest{Id: req.GetId()})
	if err != nil {
		return nil, err
	}
	_, err = l.eventClient.Publish(ctx, &events.PublishRequest{Event: event.NewEventFor(req.GetId(), events.EventType_NodeForget)})
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
