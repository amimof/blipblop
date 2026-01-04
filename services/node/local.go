package node

import (
	"context"
	"errors"
	"fmt"
	"sync"

	eventsv1 "github.com/amimof/voiyd/api/services/events/v1"
	"github.com/amimof/voiyd/api/services/nodes/v1"
	"github.com/amimof/voiyd/pkg/events"
	"github.com/amimof/voiyd/pkg/logger"
	"github.com/amimof/voiyd/pkg/protoutils"
	"github.com/amimof/voiyd/pkg/repository"
	"go.opentelemetry.io/otel"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type local struct {
	repo     repository.NodeRepository
	mu       sync.Mutex
	exchange *events.Exchange
	logger   logger.Logger
}

var (
	_      nodes.NodeServiceClient = &local{}
	tracer                         = otel.GetTracerProvider().Tracer("voiyd-server")
)

func (l *local) handleError(err error, msg string, keysAndValues ...any) error {
	def := []any{"error", err.Error()}
	def = append(def, keysAndValues...)
	l.logger.Error(msg, def...)
	if errors.Is(err, repository.ErrNotFound) {
		return status.Error(codes.NotFound, err.Error())
	}
	return status.Error(codes.Internal, err.Error())
}

func (l *local) Get(ctx context.Context, req *nodes.GetRequest, _ ...grpc.CallOption) (*nodes.GetResponse, error) {
	ctx, span := tracer.Start(ctx, "node.Get")
	defer span.End()

	node, err := l.Repo().Get(ctx, req.Id)
	if err != nil {
		return nil, l.handleError(err, "couldn't GET node from repo", "name", req.GetId())
	}
	return &nodes.GetResponse{
		Node: node,
	}, nil
}

func merge(base, patch *nodes.Node) *nodes.Node {
	return protoutils.StrategicMerge(base, patch)
}

func applyMaskedUpdate(dst, src *nodes.Status, mask *fieldmaskpb.FieldMask) error {
	if mask == nil || len(mask.Paths) == 0 {
		return status.Error(codes.InvalidArgument, "update_mask is required")
	}
	if dst == nil || src == nil {
		return status.Error(codes.InvalidArgument, "src or dst cannot be empty")
	}

	for _, p := range mask.Paths {
		switch p {
		case "phase":
			if src.Phase == nil {
				continue
			}
			dst.Phase = src.Phase
		case "status":
			if src.Status == nil {
				continue
			}
			dst.Status = src.Status
		case "hostname":
			if src.Hostname == nil {
				continue
			}
			dst.Hostname = src.Hostname
		case "runtime":
			if src.Runtime == nil {
				continue
			}
			dst.Runtime = src.Runtime
		case "version":
			if src.Version == nil {
				continue
			}
			dst.Version = src.Version
		case "ip.dns":
			if src.Ip.Dns == nil {
				continue
			}
			dst.Ip.Dns = src.Ip.Dns
		case "ip.links":
			if src.Ip.Links == nil {
				continue
			}
			dst.Ip.Links = src.Ip.Links
		case "ip.addresses":
			if src.Ip.Addresses == nil {
				continue
			}
			dst.Ip.Addresses = src.Ip.Addresses
		default:
			return fmt.Errorf("unknown mask path %q", p)
		}
	}

	return nil
}

func (l *local) Create(ctx context.Context, req *nodes.CreateRequest, _ ...grpc.CallOption) (*nodes.CreateResponse, error) {
	ctx, span := tracer.Start(ctx, "node.Create")
	defer span.End()

	l.mu.Lock()
	defer l.mu.Unlock()

	node := req.GetNode()
	if existing, _ := l.Repo().Get(ctx, node.GetMeta().GetName()); existing != nil {
		return nil, status.Error(codes.AlreadyExists, "node already exists")
	}

	nodeID := node.GetMeta().GetName()
	node.GetMeta().Created = timestamppb.Now()
	node.GetMeta().Updated = timestamppb.Now()
	node.GetMeta().Revision = 1

	// Initialize status field if empty
	if node.GetStatus() == nil {
		node.Status = &nodes.Status{}
	}

	err := l.Repo().Create(ctx, node)
	if err != nil {
		return nil, l.handleError(err, "couldn't CREATE node in repo", "name", nodeID)
	}

	// err = l.exchange.Publish(ctx, events.NewRequest(eventsv1.EventType_NodeCreate, node))
	err = l.exchange.Publish(ctx, events.NodeCreate, events.NewEvent(eventsv1.EventType_NodeCreate, node))
	if err != nil {
		return nil, l.handleError(err, "error publishing CREATE event", "name", nodeID, "event", "NodeCreate")
	}
	return &nodes.CreateResponse{
		Node: node,
	}, nil
}

func (l *local) Delete(ctx context.Context, req *nodes.DeleteRequest, _ ...grpc.CallOption) (*nodes.DeleteResponse, error) {
	ctx, span := tracer.Start(ctx, "node.Delete")
	defer span.End()

	node, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	err = l.Repo().Delete(ctx, req.GetId())
	if err != nil {
		return nil, l.handleError(err, "couldn't GET node from repo", "id", req.GetId())
	}
	// err = l.exchange.Publish(ctx, events.NewRequest(eventsv1.EventType_NodeCreate, node))
	err = l.exchange.Publish(ctx, events.NodeDelete, events.NewEvent(eventsv1.EventType_NodeDelete, node))
	if err != nil {
		return nil, l.handleError(err, "error publishing DELETE event", "name", req.GetId(), "event", "ContainerDelete")
	}
	return &nodes.DeleteResponse{
		Id: req.Id,
	}, nil
}

func (l *local) List(ctx context.Context, req *nodes.ListRequest, _ ...grpc.CallOption) (*nodes.ListResponse, error) {
	ctx, span := tracer.Start(ctx, "node.List")
	defer span.End()

	nodeList, err := l.Repo().List(ctx)
	if err != nil {
		return nil, err
	}
	return &nodes.ListResponse{
		Nodes: nodeList,
	}, nil
}

func (l *local) UpdateStatus(ctx context.Context, req *nodes.UpdateStatusRequest, opts ...grpc.CallOption) (*nodes.UpdateStatusResponse, error) {
	ctx, span := tracer.Start(ctx, "node.UpdateStatus")
	defer span.End()

	// Get the existing container before updating so we can compare specs
	existingContainer, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}

	// Apply mask safely
	base := proto.Clone(existingContainer.Status).(*nodes.Status)
	if err := applyMaskedUpdate(base, req.Status, req.UpdateMask); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "bad mask: %v", err)
	}

	existingContainer.Status = base
	if err := l.Repo().Update(ctx, existingContainer); err != nil {
		return nil, err
	}

	return &nodes.UpdateStatusResponse{
		Id: existingContainer.GetMeta().GetName(),
	}, nil
}

// Patch implements nodes.NodeServiceClient.
func (l *local) Patch(ctx context.Context, req *nodes.PatchRequest, opts ...grpc.CallOption) (*nodes.PatchResponse, error) {
	ctx, span := tracer.Start(ctx, "node.Patch")
	defer span.End()

	l.mu.Lock()
	defer l.mu.Unlock()

	// Validate request

	updateNode := req.GetNode()

	// Get existing node from repo
	existing, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		return nil, l.handleError(err, "couldn't GET node from repo", "name", updateNode.GetMeta().GetName())
	}

	// Generate field mask
	genFieldMask, err := protoutils.GenerateFieldMask(existing, updateNode)
	if err != nil {
		return nil, err
	}

	// Handle partial update
	maskedUpdate, err := protoutils.ApplyFieldMaskToNewMessage(updateNode, genFieldMask)
	if err != nil {
		return nil, err
	}

	// TODO: Handle errors
	updated := maskedUpdate.(*nodes.Node)
	existing = merge(existing, updated)

	// Update the container
	err = l.Repo().Update(ctx, existing)
	if err != nil {
		return nil, l.handleError(err, "couldn't PATCH node in repo", "name", existing.GetMeta().GetName())
	}

	// Retreive the container again so that we can include it in an event
	ctr, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}

	// Only publish if spec is updated
	if !proto.Equal(updateNode.Config, ctr.Config) {
		err = l.exchange.Publish(ctx, eventsv1.EventType_NodePatch, events.NewEvent(eventsv1.EventType_NodePatch, ctr))
		if err != nil {
			return nil, l.handleError(err, "error publishing PATCH event", "name", existing.GetMeta().GetName(), "event", "NodePatch")
		}
	}

	return &nodes.PatchResponse{
		Node: existing,
	}, nil
}

func (l *local) Update(ctx context.Context, req *nodes.UpdateRequest, _ ...grpc.CallOption) (*nodes.UpdateResponse, error) {
	ctx, span := tracer.Start(ctx, "node.Update")
	defer span.End()

	l.mu.Lock()
	defer l.mu.Unlock()

	updateNode := req.GetNode()

	// Get the existing node before updating so we can compare specs
	existingNode, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		return nil, l.handleError(err, "couldn't GET node from repo", "name", updateNode.GetMeta().GetName())
	}

	// Ignore fields
	updateNode.Status = existingNode.Status
	updateNode.GetMeta().Updated = existingNode.Meta.Updated
	updateNode.GetMeta().Created = existingNode.Meta.Created
	updateNode.GetMeta().Revision = existingNode.Meta.Revision

	updVal := protoreflect.ValueOfMessage(updateNode.GetConfig().ProtoReflect())
	newVal := protoreflect.ValueOfMessage(existingNode.GetConfig().ProtoReflect())

	// Only update metadata fields if spec is updated
	if !updVal.Equal(newVal) {
		updateNode.Meta.Revision++
		updateNode.Meta.Updated = timestamppb.Now()
	}

	// Update the node
	err = l.Repo().Update(ctx, updateNode)
	if err != nil {
		return nil, l.handleError(err, "couldn't UPDATE node in repo", "name", updateNode.GetMeta().GetName())
	}

	// Retreive the container again so that we can include it in an event
	node, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}

	// Only publish if spec is updated
	if !updVal.Equal(newVal) {
		l.logger.Debug("node was updated, emitting event to listeners", "event", "NodeUpdate", "name", node.GetMeta().GetName(), "revision", updateNode.GetMeta().GetRevision())
		err = l.exchange.Publish(ctx, eventsv1.EventType_NodeUpdate, events.NewEvent(eventsv1.EventType_NodeUpdate, node))
		if err != nil {
			return nil, l.handleError(err, "error publishing UPDATE event", "name", node.GetMeta().GetName(), "event", "NodeUpdate")
		}
	}

	return &nodes.UpdateResponse{
		Node: node,
	}, nil
}

func (l *local) Join(ctx context.Context, req *nodes.JoinRequest, _ ...grpc.CallOption) (*nodes.JoinResponse, error) {
	ctx, span := tracer.Start(ctx, "node.Join")
	defer span.End()

	nodeID := req.GetNode().GetMeta().GetName()

	node, err := l.Repo().Get(ctx, nodeID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			l.logger.Debug("creating node that jonied", "nodeID", nodeID)
			if _, err := l.Create(ctx, &nodes.CreateRequest{Node: req.GetNode()}); err != nil {
				return nil, l.handleError(err, "couldn't CREATE node", "name", nodeID)
			}
		} else {
			return nil, l.handleError(err, "couldn't GET node", "name", nodeID)
		}
	}

	// Perform update if node exists
	if err == nil {
		l.logger.Debug("updating node that jonied", "nodeID", nodeID)
		res, err := l.Update(ctx, &nodes.UpdateRequest{Id: nodeID, Node: req.GetNode()})
		if err != nil {
			return nil, err
		}
		if res != nil {
			node = res.GetNode()
		}
	}

	return &nodes.JoinResponse{
		Id: req.GetNode().GetMeta().GetName(),
	}, l.exchange.Publish(ctx, eventsv1.EventType_NodeJoin, events.NewEvent(eventsv1.EventType_NodeJoin, node))
}

func (l *local) Forget(ctx context.Context, req *nodes.ForgetRequest, _ ...grpc.CallOption) (*nodes.ForgetResponse, error) {
	ctx, span := tracer.Start(ctx, "node.Forget")
	defer span.End()

	node, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}

	_, err = l.Delete(ctx, &nodes.DeleteRequest{Id: req.GetId()})
	if err != nil {
		return nil, l.handleError(err, "couldn't FORGET node", "name", req.GetId())
	}
	err = l.exchange.Publish(ctx, events.NodeForget, events.NewEvent(eventsv1.EventType_NodeForget, node))
	if err != nil {
		return nil, l.handleError(err, "error publishing FORGET event", "name", req.GetId(), "event", "NodeForget")
	}
	return &nodes.ForgetResponse{
		Id: req.Id,
	}, nil
}

func (l *local) Connect(ctx context.Context, opt ...grpc.CallOption) (nodes.NodeService_ConnectClient, error) {
	return nil, nil
}

func (l *local) Upgrade(ctx context.Context, req *nodes.UpgradeRequest, _ ...grpc.CallOption) (*nodes.UpgradeResponse, error) {
	ctx, span := tracer.Start(ctx, "node.Upgrade")
	defer span.End()

	err := l.exchange.Publish(ctx, events.NodeUpgrade, events.NewEvent(eventsv1.EventType_NodeUpgrade, req))
	if err != nil {
		return nil, l.handleError(err, "error publishing UPGRADE event", "name", req.GetNodeSelector(), "event", "NodeUpgrade")
	}

	return &nodes.UpgradeResponse{}, nil
}

func (l *local) Repo() repository.NodeRepository {
	if l.repo != nil {
		return l.repo
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return repository.NewNodeInMemRepo()
}
