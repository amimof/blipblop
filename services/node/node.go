package node

import (
	"context"
	"io"
	"sync"
	"time"

	eventsv1 "github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/api/services/nodes/v1"
	metav1 "github.com/amimof/blipblop/api/types/v1"
	"github.com/amimof/blipblop/pkg/events"
	"github.com/amimof/blipblop/pkg/logger"
	"github.com/amimof/blipblop/pkg/repository"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

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
	nodes.UnimplementedNodeServiceServer
	local    nodes.NodeServiceClient
	logger   logger.Logger
	exchange *events.Exchange
	streams  map[string]nodes.NodeService_ConnectServer
	mu       sync.Mutex
}

func (n *NodeService) Register(server *grpc.Server) error {
	server.RegisterService(&nodes.NodeService_ServiceDesc, n)
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

func (n *NodeService) Connect(stream nodes.NodeService_ConnectServer) error {
	var nodeName string
	if md, ok := metadata.FromIncomingContext(stream.Context()); ok {
		if res, ok := md["blipblop_node_name"]; ok && len(res) > 0 {
			nodeName = res[0]
		}
	}

	// Return if no node name was found in context
	if nodeName == "" {
		return status.Error(codes.FailedPrecondition, "missing blipblop_node_name in context")
	}

	n.mu.Lock()
	n.streams[nodeName] = stream
	n.mu.Unlock()

	defer func() {
		n.logger.Info("[NODE] disappeard", "node", nodeName)
		delete(n.streams, nodeName)
	}()

	// Periodically send message to clients
	go func() {
		for {
			stream.Send(&eventsv1.Event{Type: eventsv1.EventType_NodeJoin})
			time.Sleep(3 * time.Second)
		}
	}()

	ctx := stream.Context()

	for {
		select {
		case <-ctx.Done():
			n.logger.Error("[NODE] client disconnected", "node", nodeName)
			return ctx.Err()
		default:
			msg, err := stream.Recv()
			if err == io.EOF {
				n.logger.Error("[NODE] error receving from stream, stream probably closed", "node", nodeName)
				return nil
			}

			if err != nil {
				n.logger.Error("[NODE] error receving data from stream", "error", err, "node", nodeName)
				return err
			}

			n.exchange.Publish(context.Background(), &eventsv1.PublishRequest{Event: msg})
		}
	}
}

func (n *NodeService) subscribe(ctx context.Context) {
	ch, _ := n.exchange.Subscribe(ctx)

	handlers := events.NodeEventHandlerFuncs{
		OnForget: func(e *eventsv1.Event) error {
			n.logger.Debug("Got node forget, update node status", "node", e.GetObjectId())
			fm := &fieldmaskpb.FieldMask{Paths: []string{"status.state"}}
			_, err := n.Update(ctx,
				&nodes.UpdateNodeRequest{
					Id: e.GetObjectId(),
					Node: &nodes.Node{
						Status: &nodes.Status{
							State: "MISSING",
						},
						Meta: &metav1.Meta{
							Name: e.GetObjectId(),
						},
					},
					UpdateMask: fm,
				},
			)
			return err
		},
	}

	informer := events.NewNodeEventInformer(handlers)
	go informer.Run(ch)
}

func NewService(repo repository.NodeRepository, opts ...NewServiceOption) *NodeService {
	s := &NodeService{
		logger:  logger.ConsoleLogger{},
		streams: make(map[string]nodes.NodeService_ConnectServer),
	}

	for _, opt := range opts {
		opt(s)
	}

	s.local = &local{
		repo:     repo,
		exchange: s.exchange,
		logger:   s.logger,
	}

	go s.subscribe(context.Background())

	return s
}
