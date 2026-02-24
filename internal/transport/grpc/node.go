// Package node
package grpc

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/amimof/voiyd/internal/app"
	"github.com/amimof/voiyd/pkg/condition"
	"github.com/amimof/voiyd/pkg/keys"

	nodesv1 "github.com/amimof/voiyd/api/services/nodes/v1"
	typesv1 "github.com/amimof/voiyd/api/types/v1"
)

var _ nodesv1.NodeServiceServer = &NodeService{}

type NodeService struct {
	nodesv1.UnimplementedNodeServiceServer
	app *app.NodeService
}

func (n *NodeService) Register(server *grpc.Server) {
	nodesv1.RegisterNodeServiceServer(server, n)
}

func (n *NodeService) Connect(stream nodesv1.NodeService_ConnectServer) error {
	var nodeUID string
	var nodeName string
	if md, ok := metadata.FromIncomingContext(stream.Context()); ok {
		if res, ok := md["x-voiyd-node-uid"]; ok && len(res) > 0 {
			nodeUID = res[0]
		}
	}
	if md, ok := metadata.FromIncomingContext(stream.Context()); ok {
		if res, ok := md["x-voiyd-node-name"]; ok && len(res) > 0 {
			nodeName = res[0]
		}
	}

	// Create node session
	sess, err := n.app.Connect(stream.Context(), app.NodeConnectInput{
		NodeUID:  nodeUID,
		NodeName: nodeName,
	})
	if err != nil {
		return err
	}

	// Update node status to Connected.
	// Node will update to Ready once it's ready using clientset
	go func() {
		uid, err := keys.FromUIDOrName(nodeUID, nodeName)
		if err != nil {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		_ = n.app.UpdateStatus(ctx, uid, &nodesv1.Status{Phase: wrapperspb.String(string(condition.ReasonConnected))}, "phase")
	}()

	// ctx := stream.Context()
	errCh := make(chan error, 2)

	// Update node status to Disconnected once session ends and context is cancelled
	defer func() {
		if err := sess.Close(); err != nil {
			n.app.Logger.Error("error closing session", "error", err)
		}
		uid, err := keys.FromUIDOrName(nodeUID, nodeName)
		if err != nil {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		_ = n.app.UpdateStatus(ctx, uid, &nodesv1.Status{Phase: wrapperspb.String(string(condition.ReasonDisconnected))}, "phase")
	}()

	// Reader: node -> app
	go func() {
		for {
			in, err := stream.Recv()
			if err != nil {
				errCh <- err
				return
			}
			if err := sess.Handle(stream.Context(), in); err != nil {
				errCh <- err
				return
			}
		}
	}()

	// Writer: app -> node
	go func() {
		for {
			out, err := sess.Next(stream.Context())
			if err != nil {
				errCh <- err
				return
			}

			if err := stream.Send(out); err != nil {
				errCh <- err
				return
			}
		}
	}()

	<-errCh
	return err
}

func (n *NodeService) Get(ctx context.Context, req *nodesv1.GetRequest) (*nodesv1.GetResponse, error) {
	uid, err := keys.FromUIDOrName(req.GetUid(), req.GetName())
	if err != nil {
		return nil, toStatus(err)
	}
	node, err := n.app.Get(ctx, uid)
	if err != nil {
		return nil, toStatus(err)
	}
	return &nodesv1.GetResponse{Node: node}, nil
}

func (n *NodeService) Create(ctx context.Context, req *nodesv1.CreateRequest) (*nodesv1.CreateResponse, error) {
	node, err := n.app.Create(ctx, req.GetNode())
	if err != nil {
		return nil, toStatus(err)
	}
	return &nodesv1.CreateResponse{Node: node}, nil
}

func (n *NodeService) Delete(ctx context.Context, req *nodesv1.DeleteRequest) (*empty.Empty, error) {
	uid, err := keys.FromUIDOrName(req.GetUid(), req.GetName())
	if err != nil {
		return nil, toStatus(err)
	}

	err = n.app.Delete(ctx, uid)
	if err != nil {
		return nil, toStatus(err)
	}

	return &emptypb.Empty{}, nil
}

func (n *NodeService) List(ctx context.Context, req *nodesv1.ListRequest) (*nodesv1.ListResponse, error) {
	nodes, err := n.app.List(ctx, req.GetLimit())
	if err != nil {
		return nil, toStatus(err)
	}
	return &nodesv1.ListResponse{Nodes: nodes}, nil
}

func (n *NodeService) Update(ctx context.Context, req *nodesv1.UpdateRequest) (*nodesv1.UpdateResponse, error) {
	uid, err := keys.FromUIDOrName(req.GetUid(), req.GetName())
	if err != nil {
		return nil, toStatus(err)
	}

	err = n.app.Update(ctx, uid, req.GetNode())
	if err != nil {
		return nil, toStatus(err)
	}

	node, err := n.app.Get(ctx, uid)
	if err != nil {
		return nil, toStatus(err)
	}

	return &nodesv1.UpdateResponse{Node: node}, nil
}

func (n *NodeService) Patch(ctx context.Context, req *nodesv1.PatchRequest) (*nodesv1.PatchResponse, error) {
	uid, err := keys.FromUIDOrName(req.GetUid(), req.GetName())
	if err != nil {
		return nil, toStatus(err)
	}

	err = n.app.Patch(ctx, uid, req.GetNode())
	if err != nil {
		return nil, toStatus(err)
	}

	node, err := n.app.Get(ctx, uid)
	if err != nil {
		return nil, toStatus(err)
	}

	return &nodesv1.PatchResponse{Node: node}, nil
}

func (n *NodeService) UpdateStatus(ctx context.Context, req *nodesv1.UpdateStatusRequest) (*nodesv1.UpdateStatusResponse, error) {
	uid, err := keys.FromUIDOrName(req.GetUid(), req.GetName())
	if err != nil {
		return nil, toStatus(err)
	}

	err = n.app.UpdateStatus(ctx, uid, req.GetStatus(), req.GetUpdateMask().GetPaths()...)
	if err != nil {
		return nil, toStatus(err)
	}

	return &nodesv1.UpdateStatusResponse{Id: uid.UUIDStr()}, nil
}

func (n *NodeService) Condition(ctx context.Context, req *typesv1.ConditionRequest) (*emptypb.Empty, error) {
	uid, err := keys.ParseStr(req.GetTaskId())
	if err != nil {
		return nil, toStatus(err)
	}
	err = n.app.Condition(ctx, uid, req)
	if err != nil {
		return nil, toStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (n *NodeService) Upgrade(ctx context.Context, req *nodesv1.UpgradeRequest) (*nodesv1.UpgradeResponse, error) {
	return &nodesv1.UpgradeResponse{}, n.app.Upgrade(ctx, req)
}

func (n *NodeService) Join(ctx context.Context, req *nodesv1.JoinRequest) (*nodesv1.JoinResponse, error) {
	uid, err := keys.FromUIDOrName(req.GetNode().GetMeta().GetUid(), req.GetNode().GetMeta().GetName())
	if err != nil {
		return nil, toStatus(err)
	}

	node, err := n.app.Join(ctx, uid, req.GetNode())
	if err != nil {
		return nil, toStatus(err)
	}

	return &nodesv1.JoinResponse{Uid: node.GetMeta().GetUid(), Name: node.GetMeta().GetName()}, nil
}

func (n *NodeService) Forget(ctx context.Context, req *nodesv1.ForgetRequest) (*nodesv1.ForgetResponse, error) {
	uid, err := keys.FromUIDOrName(req.GetUid(), req.GetName())
	if err != nil {
		return nil, toStatus(err)
	}
	err = n.app.Forget(ctx, uid)
	if err != nil {
		return nil, toStatus(err)
	}

	return &nodesv1.ForgetResponse{Id: uid.UUIDStr()}, nil
}

func NewNodeService(app *app.NodeService) *NodeService {
	return &NodeService{app: app}
}
