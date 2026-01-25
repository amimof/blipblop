// Package node
package node

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/amimof/voiyd/pkg/condition"
	"github.com/amimof/voiyd/pkg/events"
	"github.com/amimof/voiyd/pkg/logger"
	"github.com/amimof/voiyd/pkg/repository"

	nodesv1 "github.com/amimof/voiyd/api/services/nodes/v1"
	typesv1 "github.com/amimof/voiyd/api/types/v1"
)

var _ nodesv1.NodeServiceServer = &NodeService{}

const Version string = "node/v1"

type NewServiceOption func(s *NodeService)

func WithLogger(l logger.Logger) NewServiceOption {
	return func(s *NodeService) {
		s.logger = l
	}
}

func WithExchange(e *events.Exchange) NewServiceOption {
	return func(s *NodeService) {
		s.exchange = e
	}
}

type NodeService struct {
	nodesv1.UnimplementedNodeServiceServer
	local       nodesv1.NodeServiceClient
	logger      logger.Logger
	exchange    *events.Exchange
	streams     map[string]nodesv1.NodeService_ConnectServer
	mu          sync.Mutex
	logExchange *events.LogExchange
}

func (n *NodeService) Register(server *grpc.Server) error {
	server.RegisterService(&nodesv1.NodeService_ServiceDesc, n)
	return nil
}

func (n *NodeService) Get(ctx context.Context, req *nodesv1.GetRequest) (*nodesv1.GetResponse, error) {
	return n.local.Get(ctx, req)
}

func (n *NodeService) Create(ctx context.Context, req *nodesv1.CreateRequest) (*nodesv1.CreateResponse, error) {
	return n.local.Create(ctx, req)
}

func (n *NodeService) Delete(ctx context.Context, req *nodesv1.DeleteRequest) (*nodesv1.DeleteResponse, error) {
	n.closeNodeStream(req.GetId())
	return n.local.Delete(ctx, req)
}

func (n *NodeService) List(ctx context.Context, req *nodesv1.ListRequest) (*nodesv1.ListResponse, error) {
	return n.local.List(ctx, req)
}

func (n *NodeService) Update(ctx context.Context, req *nodesv1.UpdateRequest) (*nodesv1.UpdateResponse, error) {
	return n.local.Update(ctx, req)
}

func (n *NodeService) Patch(ctx context.Context, req *nodesv1.PatchRequest) (*nodesv1.PatchResponse, error) {
	return n.local.Patch(ctx, req)
}

func (n *NodeService) Join(ctx context.Context, req *nodesv1.JoinRequest) (*nodesv1.JoinResponse, error) {
	return n.local.Join(ctx, req)
}

func (n *NodeService) Forget(ctx context.Context, req *nodesv1.ForgetRequest) (*nodesv1.ForgetResponse, error) {
	n.closeNodeStream(req.GetId())
	return n.local.Forget(ctx, req)
}

func (n *NodeService) UpdateStatus(ctx context.Context, req *nodesv1.UpdateStatusRequest) (*nodesv1.UpdateStatusResponse, error) {
	return n.local.UpdateStatus(ctx, req)
}

func (n *NodeService) Upgrade(ctx context.Context, req *nodesv1.UpgradeRequest) (*nodesv1.UpgradeResponse, error) {
	return n.local.Upgrade(ctx, req)
}

func (n *NodeService) Condition(ctx context.Context, req *typesv1.ConditionRequest) (*emptypb.Empty, error) {
	return n.local.Condition(ctx, req)
}

func (n *NodeService) Connect(stream nodesv1.NodeService_ConnectServer) error {
	ctx := stream.Context()

	var nodeName string
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if res, ok := md["voiyd_node_name"]; ok && len(res) > 0 {
			nodeName = res[0]
		}
	}

	// Return if no node name was found in context
	if nodeName == "" {
		return status.Error(codes.FailedPrecondition, "missing voiyd_node_name in context")
	}

	// Check if node is joined to cluster prior to connecting
	res, err := n.Get(ctx, &nodesv1.GetRequest{Id: nodeName})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return errors.Join(fmt.Errorf("node %s not found", nodeName), err)
		}
		return err
	}

	node := res.GetNode()

	n.mu.Lock()
	n.streams[nodeName] = stream
	n.mu.Unlock()

	// Publish event that node is connected
	err = n.exchange.Publish(ctx, events.NewEvent(events.NodeConnect, node))
	if err != nil {
		n.logger.Error("error publishing NodeConnect event", "error", err)
	}

	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		reporter := condition.NewForResource(node)

		n.logger.Info("removing node stream", "node", nodeName)

		// Report disconnected status with fresh context
		if _, err = n.Condition(cleanupCtx, &typesv1.ConditionRequest{
			ResourceVersion: Version,
			Report:          reporter.Type(condition.NodeReady).False(condition.ReasonDisconnected),
		}); err != nil {
			n.logger.Error("failed to report node disconnection", "error", err, "node", nodeName)
		} else {
			n.logger.Info("successfully reported node disconnection", "node", nodeName)
		}

		// Publish disconnect event
		if err = n.exchange.Publish(cleanupCtx, events.NewEvent(events.NodeForget, node)); err != nil {
			n.logger.Error("error publishing node forget event", "error", err)
		}

		// Remove stream from map
		n.mu.Lock()
		delete(n.streams, nodeName)
		n.mu.Unlock()
	}()

	for {
		select {
		case <-ctx.Done():
			n.logger.Info("node stream context cancelled", "node", nodeName, "reason", ctx.Err())
			return ctx.Err()
		default:
			msg, err := stream.Recv()
			if err == io.EOF {
				n.logger.Info("node stream closed (EOF)", "node", nodeName)
				return nil
			}

			if err != nil {
				n.logger.Error("node stream error", "error", err, "node", nodeName)
				return err
			}

			err = n.exchange.Publish(ctx, msg)
			if err != nil {
				return err
			}
		}
	}
}

func (n *NodeService) closeNodeStream(nodeName string) {
	n.mu.Lock()
	defer n.mu.Unlock()

	_, exists := n.streams[nodeName]
	if !exists {
		return
	}

	n.logger.Info("closing stream for deleted node", "node", nodeName)

	// Note: We don't need to explicitly close the stream or call stream.Context().Cancel()
	// The defer in Connect() will handle cleanup when the stream ends.
	// We just remove it from our map to prevent future sends.
	delete(n.streams, nodeName)

	// The stream will naturally close when the client's context is done
	// or when the next Recv()/Send() operation fails
}

func NewService(repo repository.NodeRepository, opts ...NewServiceOption) *NodeService {
	s := &NodeService{
		logger:      logger.ConsoleLogger{},
		streams:     make(map[string]nodesv1.NodeService_ConnectServer),
		logExchange: &events.LogExchange{},
	}

	for _, opt := range opts {
		opt(s)
	}

	s.local = &local{
		repo:     repo,
		exchange: s.exchange,
		logger:   s.logger,
	}

	return s
}
