package node

import (
	"context"
	"errors"
	"sync"

	eventsv1 "github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/api/services/nodes/v1"
	"github.com/amimof/blipblop/pkg/events"
	"github.com/amimof/blipblop/pkg/logger"
	"github.com/amimof/blipblop/pkg/protoutils"
	"github.com/amimof/blipblop/pkg/repository"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type local struct {
	repo     repository.NodeRepository
	mu       sync.Mutex
	exchange *events.Exchange
	logger   logger.Logger
}

var _ nodes.NodeServiceClient = &local{}

func (l *local) handleError(err error, msg string, keysAndValues ...any) error {
	def := []any{"error", err.Error()}
	def = append(def, keysAndValues...)
	l.logger.Error(msg, def...)
	if errors.Is(err, repository.ErrNotFound) {
		return status.Error(codes.NotFound, err.Error())
	}
	return status.Error(codes.Internal, err.Error())
}

func (l *local) Get(ctx context.Context, req *nodes.GetNodeRequest, _ ...grpc.CallOption) (*nodes.GetNodeResponse, error) {
	node, err := l.Repo().Get(ctx, req.Id)
	if err != nil {
		return nil, l.handleError(err, "couldn't GET node from repo", "name", req.GetId())
	}
	return &nodes.GetNodeResponse{
		Node: node,
	}, nil
}

func (l *local) Create(ctx context.Context, req *nodes.CreateNodeRequest, _ ...grpc.CallOption) (*nodes.CreateNodeResponse, error) {
	node := req.GetNode()
	if existing, _ := l.Repo().Get(ctx, node.GetMeta().GetName()); existing != nil {
		return nil, status.Error(codes.AlreadyExists, "node already exists")
	}

	nodeId := node.Meta.GetName()
	node.Meta.Created = timestamppb.Now()

	err := req.Validate()
	if err != nil {
		return nil, err
	}

	err = l.Repo().Create(ctx, node)
	if err != nil {
		return nil, l.handleError(err, "couldn't CREATE node in repo", "name", nodeId)
	}

	err = l.exchange.Publish(ctx, events.NewRequest(eventsv1.EventType_NodeCreate, node))
	if err != nil {
		return nil, l.handleError(err, "error publishing CREATE event", "name", nodeId, "event", "NodeCreate")
	}
	return &nodes.CreateNodeResponse{
		Node: node,
	}, nil
}

func (l *local) Delete(ctx context.Context, req *nodes.DeleteNodeRequest, _ ...grpc.CallOption) (*nodes.DeleteNodeResponse, error) {
	node, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	err = l.Repo().Delete(ctx, req.GetId())
	if err != nil {
		return nil, l.handleError(err, "couldn't GET node from repo", "id", req.GetId())
	}
	err = l.exchange.Publish(ctx, events.NewRequest(eventsv1.EventType_NodeCreate, node))
	if err != nil {
		return nil, l.handleError(err, "error publishing DELETE event", "name", req.GetId(), "event", "ContainerDelete")
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
	return &nodes.ListNodeResponse{
		Nodes: nodeList,
	}, nil
}

func (l *local) Update(ctx context.Context, req *nodes.UpdateNodeRequest, _ ...grpc.CallOption) (*nodes.UpdateNodeResponse, error) {
	updateMask := req.GetUpdateMask()
	updateNode := req.GetNode()
	existing, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		return nil, l.handleError(err, "couldn't GET node from repo", "name", updateNode.GetMeta().GetName())
	}

	// Apply the FieldMask selectively
	maskedUpdate, err := protoutils.ApplyFieldMaskToNewMessage(updateNode, updateMask)
	if err != nil {
		return nil, err
	}
	proto.Merge(existing, maskedUpdate)

	existing.GetMeta().Updated = timestamppb.Now()

	err = existing.Validate()
	if err != nil {
		return nil, err
	}

	err = l.Repo().Update(ctx, existing)
	if err != nil {
		return nil, l.handleError(err, "couldn't UPDATE node in repo", "name", existing.GetMeta().GetName())
	}

	node, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}

	err = l.exchange.Publish(ctx, events.NewRequest(eventsv1.EventType_NodeCreate, node))
	if err != nil {
		return nil, l.handleError(err, "error publishing UPDATE event", "name", existing.GetMeta().GetName(), "event", "NodeUpdate")
	}
	return &nodes.UpdateNodeResponse{
		Node: existing,
	}, nil
}

func (l *local) Join(ctx context.Context, req *nodes.JoinRequest, _ ...grpc.CallOption) (*nodes.JoinResponse, error) {
	nodeId := req.GetNode().GetMeta().GetName()

	if nodeId == "" {
		return nil, status.Error(codes.InvalidArgument, "node name cannot be empty")
	}

	node, err := l.Repo().Get(ctx, nodeId)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			if _, err := l.Create(ctx, &nodes.CreateNodeRequest{Node: req.Node}); err != nil {
				return nil, l.handleError(err, "couldn't CREATE node", "name", nodeId)
			}
		} else {
			return nil, l.handleError(err, "couldn't GET node", "name", nodeId)
		}
	}

	res, err := l.Update(ctx, &nodes.UpdateNodeRequest{Id: nodeId, Node: req.GetNode()})
	if err != nil {
		return nil, err
	}

	if res != nil {
		node = res.GetNode()
	}

	return &nodes.JoinResponse{
		Id: req.GetNode().GetMeta().GetName(),
	}, l.exchange.Publish(ctx, events.NewRequest(eventsv1.EventType_NodeJoin, node))
}

func (l *local) Forget(ctx context.Context, req *nodes.ForgetRequest, _ ...grpc.CallOption) (*nodes.ForgetResponse, error) {
	node, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}

	_, err = l.Delete(ctx, &nodes.DeleteNodeRequest{Id: req.GetId()})
	if err != nil {
		return nil, l.handleError(err, "couldn't FORGET node", "name", req.GetId())
	}
	err = l.exchange.Publish(ctx, events.NewRequest(eventsv1.EventType_NodeForget, node))
	if err != nil {
		return nil, l.handleError(err, "error publishing FORGET event", "name", req.GetId(), "event", "NodeForget")
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
