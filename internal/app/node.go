package app

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	// errs "github.com/amimof/voiyd/pkg/errors"

	"github.com/amimof/voiyd/pkg/events"
	"github.com/amimof/voiyd/pkg/keys"
	"github.com/amimof/voiyd/pkg/labels"
	"github.com/amimof/voiyd/pkg/logger"
	"github.com/amimof/voiyd/pkg/protoutils"
	"github.com/amimof/voiyd/pkg/repository"

	// noderepo "github.com/amimof/voiyd/pkg/repository/node"

	nodesv1 "github.com/amimof/voiyd/api/services/nodes/v1"
	typesv1 "github.com/amimof/voiyd/api/types/v1"
)

type NodeService struct {
	Repo     *repository.Repo[*nodesv1.Node]
	mu       sync.Mutex
	Exchange *events.Exchange
	Logger   logger.Logger
	Manager  SessionManager
	Sender   NodeSender
}

func (l *NodeService) handleError(err error, msg string, keysAndValues ...any) error {
	def := []any{"error", err.Error()}
	def = append(def, keysAndValues...)
	l.Logger.Error(msg, def...)
	if errors.Is(err, repository.ErrNotFound) {
		return status.Error(codes.NotFound, err.Error())
	}
	return status.Error(codes.Internal, err.Error())
}

func applyMaskedUpdateNode(dst, src *nodesv1.Status, mask *fieldmaskpb.FieldMask) error {
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
		case "conditions":
			if src.Conditions == nil {
				continue
			}
			dst.Conditions = src.Conditions
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

func (l *NodeService) Get(ctx context.Context, id keys.ID) (*nodesv1.Node, error) {
	ctx, span := tracer.Start(ctx, "node.Get")
	defer span.End()

	return l.Repo.Get(ctx, id)
}

func (l *NodeService) Create(ctx context.Context, node *nodesv1.Node) (*nodesv1.Node, error) {
	ctx, span := tracer.Start(ctx, "node.Create")
	defer span.End()

	l.mu.Lock()
	defer l.mu.Unlock()

	newNode, err := l.Repo.Create(ctx, node)
	if err != nil {
		return nil, l.handleError(err, "error creating node", "name", newNode.GetMeta().GetName())
	}

	err = l.Exchange.Forward(ctx, events.NewEvent(events.NodeCreate, newNode))
	if err != nil {
		return nil, l.handleError(err, "error publishing node create event", "name", newNode.GetMeta().GetName())
	}

	return newNode, nil
}

func (l *NodeService) Delete(ctx context.Context, id keys.ID) error {
	ctx, span := tracer.Start(ctx, "node.Delete")
	defer span.End()

	l.mu.Lock()
	defer l.mu.Unlock()

	node, err := l.Repo.Get(ctx, id)
	if err != nil {
		return err
	}

	err = l.Repo.Delete(ctx, id)
	if err != nil {
		return err
	}

	err = l.Exchange.Forward(ctx, events.NewEvent(events.NodeDelete, node))
	if err != nil {
		return l.handleError(err, "error publishing node delete event", "name", node.GetMeta().GetName())
	}

	return nil
}

func (l *NodeService) List(ctx context.Context, limit int32) ([]*nodesv1.Node, error) {
	ctx, span := tracer.Start(ctx, "node.List")
	defer span.End()

	return l.Repo.List(ctx, limit)
}

func (l *NodeService) UpdateStatus(ctx context.Context, id keys.ID, st *nodesv1.Status, mask ...string) error {
	ctx, span := tracer.Start(ctx, "node.UpdateStatus")
	defer span.End()

	l.mu.Lock()
	defer l.mu.Unlock()

	existingNode, err := l.Repo.Get(ctx, id)
	if err != nil {
		return err
	}

	if existingNode.GetStatus() == nil {
		existingNode.Status = &nodesv1.Status{}
	}

	// Apply mask safely
	base := proto.Clone(existingNode.Status).(*nodesv1.Status)
	if err := applyMaskedUpdateNode(base, st, &fieldmaskpb.FieldMask{Paths: mask}); err != nil {
		return status.Errorf(codes.InvalidArgument, "bad mask: %v", err)
	}

	existingNode.Status = base

	if _, err := l.Repo.Update(ctx, id, existingNode); err != nil {
		return err
	}

	return nil
}

func (l *NodeService) Patch(ctx context.Context, id keys.ID, patch *nodesv1.Node) error {
	ctx, span := tracer.Start(ctx, "node.Patch")
	defer span.End()

	l.mu.Lock()
	defer l.mu.Unlock()

	// Get existing node from repo
	existing, err := l.Repo.Get(ctx, id)
	if err != nil {
		return l.handleError(err, "error getting node", "name", patch.GetMeta().GetName())
	}

	// Generate field mask
	genFieldMask, err := protoutils.GenerateFieldMask(existing, patch)
	if err != nil {
		return err
	}

	// Handle partial update
	maskedUpdate, err := protoutils.ApplyFieldMaskToNewMessage(patch, genFieldMask)
	if err != nil {
		return err
	}

	updated := maskedUpdate.(*nodesv1.Node)
	existing = protoutils.StrategicMerge(existing, updated)

	// Update the node
	node, err := l.Repo.Update(ctx, id, existing)
	if err != nil {
		return l.handleError(err, "error patching node", "name", existing.GetMeta().GetName())
	}

	changed, err := protoutils.SpecEqual(existing.GetConfig(), node.GetConfig())
	if err != nil {
		return err
	}

	// Only publish if spec is updated
	if changed {
		err = l.Exchange.Forward(ctx, events.NewEvent(events.NodePatch, node))
		if err != nil {
			return l.handleError(err, "error publishing node patch event", "name", existing.GetMeta().GetName())
		}
	}

	return nil
}

func (l *NodeService) Update(ctx context.Context, id keys.ID, node *nodesv1.Node) error {
	ctx, span := tracer.Start(ctx, "node.Update")
	defer span.End()

	l.mu.Lock()
	defer l.mu.Unlock()

	// Get the existing node before updating so we can compare specs
	existingNode, err := l.Repo.Get(ctx, id)
	if err != nil {
		return l.handleError(err, "error getting node", "name", node.GetMeta().GetName())
	}

	// Update the node
	updated, err := l.Repo.Update(ctx, id, node)
	if err != nil {
		return l.handleError(err, "error updating node", "name", node.GetMeta().GetName())
	}

	// Notify session manager about the change
	err = l.Manager.Set(ctx, updated.GetMeta().GetUid(), node)
	if err != nil {
		return err
	}

	changed, err := protoutils.SpecEqual(existingNode.GetConfig(), updated.GetConfig())
	if err != nil {
		return err
	}

	// Only publish if spec is updated
	if changed {
		err = l.Exchange.Forward(ctx, events.NewEvent(events.NodeUpdate, updated))
		if err != nil {
			return l.handleError(err, "error publishing node update event", "name", updated.GetMeta().GetName())
		}
	}

	return nil
}

func (l *NodeService) Join(ctx context.Context, id keys.ID, node *nodesv1.Node) (*nodesv1.Node, error) {
	ctx, span := tracer.Start(ctx, "node.Join")
	defer span.End()

	_, err := l.Repo.Get(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			l.Logger.Debug("creating node that joined", "nodeID", node.GetMeta().GetName())
			if _, err := l.Create(ctx, node); err != nil {
				return nil, l.handleError(err, "error creating node", "name", node.GetMeta().GetName())
			}
		}
		return nil, l.handleError(err, "error getting node", "name", node.GetMeta().GetName())
	}

	// Perform update if node exists
	l.Logger.Debug("updating node that joined", "nodeID", node.GetMeta().GetName())

	_, err = l.Repo.Update(ctx, id, node)
	if err != nil {
		return nil, err
	}

	err = l.Exchange.Forward(ctx, events.NewEvent(events.NodeJoin, node))
	if err != nil {
		return nil, l.handleError(err, "error publishing node join event", "name", node.GetMeta().GetName())
	}

	return l.Get(ctx, id)
}

func (l *NodeService) Forget(ctx context.Context, id keys.ID) error {
	ctx, span := tracer.Start(ctx, "node.Forget")
	defer span.End()

	l.mu.Lock()
	defer l.mu.Unlock()

	node, err := l.Repo.Get(ctx, id)
	if err != nil {
		return err
	}

	err = l.Repo.Delete(ctx, id)
	if err != nil {
		return l.handleError(err, "error deleting node", "name", node.GetMeta().GetName())
	}

	err = l.Exchange.Forward(ctx, events.NewEvent(events.NodeForget, node))
	if err != nil {
		return l.handleError(err, "error publishing node forget event", "name", node.GetMeta().GetName())
	}

	return nil
}

func (l *NodeService) Connect(ctx context.Context, in NodeConnectInput) (Session, error) {
	id, err := keys.FromUIDOrName(in.NodeUID, in.NodeName)
	if err != nil {
		return nil, err
	}
	res, err := l.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return l.Manager.Connect(ctx, res, in)
}

func (l *NodeService) Upgrade(ctx context.Context, req *nodesv1.UpgradeRequest) error {
	ctx, span := tracer.Start(ctx, "node.Upgrade")
	defer span.End()

	l.mu.Lock()
	defer l.mu.Unlock()

	// Send to all nodes matching selector
	if req.GetSelector() != nil {
		nodes, err := l.Repo.List(ctx, 0)
		if err != nil {
			return err
		}
		for _, n := range nodes {
			selector := labels.NewCompositeSelectorFromMap(req.GetSelector())
			if selector.Matches(n.GetMeta().GetLabels()) {
				if !l.Sender.IsNodeConnected(req.GetUid()) {
					l.Logger.Warn("not sending upgrade request to disconnected", "node", n.GetMeta().GetName())
					continue
				}
				if err := l.Sender.SendToNode(ctx, n.GetMeta().GetUid(), events.NewEvent(events.NodeUpgrade, req)); err != nil {
					l.Logger.Warn("error sending upgrade request to node", "error", err, "node", n.GetMeta().GetName())
					continue
				}
			}
		}
	}

	// Send to node by uid
	if req.GetUid() != "" {
		if !l.Sender.IsNodeConnected(req.GetUid()) {
			return ErrNodeNotConnected
		}
		return l.Sender.SendToNode(ctx, req.GetUid(), events.NewEvent(events.NodeUpgrade, req))
	}

	return nil
}

func (l *NodeService) Condition(ctx context.Context, id keys.ID, req *typesv1.ConditionRequest) error {
	st := &nodesv1.Status{
		Conditions: req.GetConditions(),
	}

	err := l.UpdateStatus(ctx, id, st, "conditions")
	if err != nil {
		return err
	}

	err = l.Exchange.Publish(ctx, events.NewEvent(events.ConditionReported, req))
	if err != nil {
		return err
	}
	return nil
}
