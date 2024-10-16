package node

import (
	"context"
	"errors"
	"sync"

	"github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/api/services/nodes/v1"
	errdefs "github.com/amimof/blipblop/pkg/errors"
	"github.com/amimof/blipblop/pkg/logger"
	"github.com/amimof/blipblop/pkg/repository"
	"github.com/amimof/blipblop/services"
	"github.com/amimof/blipblop/services/event"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type local struct {
	repo        repository.NodeRepository
	mu          sync.Mutex
	eventClient *event.EventService
	logger      logger.Logger
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
	// md, _ := metadata.FromIncomingContext(ctx)
	// clientId := md.Get("blipblop_client_id")[0]
	node, err := l.Repo().Get(ctx, req.Id)
	if err != nil {
		return nil, l.handleError(err, "couldn't GET node from repo", "name", req.GetId())
	}
	// _, err = l.eventClient.Publish(ctx, &events.PublishRequest{Event: event.NewEventFor(clientId, req.GetId(), events.EventType_NodeGet)})
	// if err != nil {
	// 	return nil, l.handleError(err, "error publishing GET event", "id", req.GetId(), "event", "NodeGet")
	// }
	return &nodes.GetNodeResponse{
		Node: node,
	}, nil
}

func (l *local) Create(ctx context.Context, req *nodes.CreateNodeRequest, _ ...grpc.CallOption) (*nodes.CreateNodeResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	clientId := md.Get("blipblop_client_id")[0]
	node := req.GetNode()
	if existing, _ := l.Repo().Get(ctx, node.GetMeta().GetName()); existing != nil {
		return nil, status.Error(codes.AlreadyExists, "node already exists")
	}

	err := services.EnsureMeta(node)
	if err != nil {
		return nil, err
	}

	err = l.Repo().Create(ctx, node)
	if err != nil {
		return nil, l.handleError(err, "couldn't CREATE node in repo", "name", node.GetMeta().GetName())
	}
	_, err = l.eventClient.Publish(ctx, &events.PublishRequest{Event: event.NewEventFor(clientId, req.GetNode().GetMeta().GetName(), events.EventType_NodeCreate)})
	if err != nil {
		return nil, l.handleError(err, "error publishing CREATE event", "name", node.GetMeta().GetName(), "event", "NodeCreate")
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
		return nil, l.handleError(err, "couldn't GET node from repo", "id", req.GetId())
	}
	_, err = l.eventClient.Publish(ctx, &events.PublishRequest{Event: event.NewEventFor(clientId, req.GetId(), events.EventType_NodeDelete)})
	if err != nil {
		return nil, l.handleError(err, "error publishing DELETE event", "name", req.GetId(), "event", "ContainerDelete")
	}
	return &nodes.DeleteNodeResponse{
		Id: req.Id,
	}, nil
}

func (l *local) List(ctx context.Context, req *nodes.ListNodeRequest, _ ...grpc.CallOption) (*nodes.ListNodeResponse, error) {
	// md, _ := metadata.FromIncomingContext(ctx)
	// clientId := md.Get("blipblop_client_id")[0]
	nodeList, err := l.Repo().List(ctx)
	if err != nil {
		return nil, err
	}
	// _, err = l.eventClient.Publish(ctx, &events.PublishRequest{Event: event.NewEventFor(clientId, "", events.EventType_NodeList)})
	// if err != nil {
	// 	return nil, err
	// }

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
		return nil, l.handleError(err, "couldn't GET node from repo", "name", updateNode.GetMeta().GetName())
	}

	if updateMask != nil && updateMask.IsValid(existing) {
		proto.Merge(existing, updateNode)
	}

	err = l.Repo().Update(ctx, existing)
	if err != nil {
		return nil, l.handleError(err, "couldn't UPDATE node in repo", "name", existing.GetMeta().GetName())
	}
	_, err = l.eventClient.Publish(ctx, &events.PublishRequest{Event: event.NewEventFor(clientId, existing.GetMeta().GetName(), events.EventType_NodeUpdate)})
	if err != nil {
		return nil, l.handleError(err, "error publishing UPDATE event", "name", existing.GetMeta().GetName(), "event", "NodeUpdate")
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
		return nil, status.Error(codes.InvalidArgument, "node name cannot be empty")
	}

	_, err := l.Get(ctx, &nodes.GetNodeRequest{Id: req.GetNode().GetMeta().GetName()})
	if err != nil {
		if errdefs.IsNotFound(err) {
			if _, err := l.Create(ctx, &nodes.CreateNodeRequest{Node: req.Node}); err != nil {
				return nil, l.handleError(err, "couldn't CREATE node", "name", req.GetNode().GetMeta().GetName())
			}
		} else {
			return nil, l.handleError(err, "couldn't GET node", "name", req.GetNode().GetMeta().GetName())
		}
	}

	// if node != nil {
	// 	if _, err = l.Update(ctx, &nodes.UpdateNodeRequest{Id: req.GetNode().GetMeta().GetName(), Node: node.GetNode()}); err != nil {
	// 		return nil, l.handleError(err, "couldn't UPDATE node", "name", req.GetNode().GetMeta().GetName())
	// 	}
	// }

	_, err = l.eventClient.Publish(ctx, &events.PublishRequest{Event: event.NewEventFor(clientId, req.GetNode().GetMeta().GetName(), events.EventType_NodeJoin)})
	if err != nil {
		return nil, l.handleError(err, "error publishing JOIN event", "name", req.GetNode().GetMeta().GetName(), "event", "NodeJoin")
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
		return nil, l.handleError(err, "couldn't FORGET node", "name", req.GetId())
	}
	_, err = l.eventClient.Publish(ctx, &events.PublishRequest{Event: event.NewEventFor(clientId, req.GetId(), events.EventType_NodeForget)})
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
