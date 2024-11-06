package node

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/amimof/blipblop/api/services/containers/v1"
	eventsv1 "github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/api/services/nodes/v1"
	metav1 "github.com/amimof/blipblop/api/types/v1"
	"github.com/amimof/blipblop/pkg/events"
	"github.com/amimof/blipblop/pkg/labels"
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

	// Check if node is joined to cluster prior to connecting
	res, err := n.Get(stream.Context(), &nodes.GetNodeRequest{Id: nodeName})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return errors.Join(fmt.Errorf("node %s not found", nodeName), err)
		}
		return err
	}

	ctx := stream.Context()
	node := res.GetNode()

	n.mu.Lock()
	n.streams[nodeName] = stream
	n.mu.Unlock()

	// Publish event that node is connected
	err = n.exchange.Publish(ctx, events.NewRequest(eventsv1.EventType_NodeConnect, node))
	if err != nil {
		n.logger.Error("error publishing NodeConnect event", "error", err)
	}

	defer func() {
		n.logger.Info("removing node stream", "node", nodeName)
		fm := &fieldmaskpb.FieldMask{Paths: []string{"status.state"}}
		_, err := n.Update(ctx,
			&nodes.UpdateNodeRequest{
				Id: node.GetMeta().GetName(),
				Node: &nodes.Node{
					Status: &nodes.Status{
						State: "MISSING",
					},
					Meta: &metav1.Meta{
						Name: node.GetMeta().GetName(),
					},
				},
				UpdateMask: fm,
			},
		)
		if err != nil {
			n.logger.Error("error updating node status", "error", err, "node", nodeName)
		}
		delete(n.streams, nodeName)
	}()

	// TESTING Periodically send message to clients
	// go func() {
	// 	for {
	// 		stream.Send(&eventsv1.Event{Type: eventsv1.EventType_NodeJoin})
	// 		time.Sleep(3 * time.Second)
	// 	}
	// }()

	for {
		select {
		case <-ctx.Done():
			n.logger.Error("node disconnected", "node", nodeName)

			return ctx.Err()
		default:
			msg, err := stream.Recv()
			if err == io.EOF {
				n.logger.Error("error receving from stream, stream probably closed", "node", nodeName)
				return nil
			}

			if err != nil {
				n.logger.Error("error receving data from stream", "error", err, "node", nodeName)
				return err
			}

			n.exchange.Publish(context.Background(), &eventsv1.PublishRequest{Event: msg})
		}
	}
}

func broadcastEvent(input <-chan *eventsv1.Event, outputs ...chan *eventsv1.Event) {
	// Send the same message to each output channel
	for event := range input {
		for _, out := range outputs {
			out <- event
		}
	}

	// Close all output channels
	for _, out := range outputs {
		close(out)
	}
}

func (n *NodeService) subscribe(ctx context.Context) {
	ch, _ := n.exchange.Subscribe(ctx)

	nodeEvt := make(chan *eventsv1.Event, 10)
	ctrEvt := make(chan *eventsv1.Event, 10)

	go broadcastEvent(ch, nodeEvt, ctrEvt)

	nodeHandlers := events.NodeEventHandlerFuncs{
		OnForget: n.onForget,
	}
	nodeinformer := events.NewNodeEventInformer(nodeHandlers)
	go nodeinformer.Run(nodeEvt)

	containerHandlers := events.ContainerEventHandlerFuncs{
		OnCreate: n.onContainerCreate,
		OnDelete: n.onContainerDelete,
	}

	containerInformer := events.NewContainerEventInformer(containerHandlers)
	go containerInformer.Run(ctrEvt)
}

func (n *NodeService) onForget(e *eventsv1.Event) error {
	n.logger.Debug("got node forget, update node status", "node", e.GetObjectId())
	fm := &fieldmaskpb.FieldMask{Paths: []string{"status.state"}}
	req := &nodes.UpdateNodeRequest{
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
	}
	_, err := n.Update(context.Background(), req)
	return err
}

func (n *NodeService) onContainerCreate(e *eventsv1.Event) error {
	n.logger.Info("node service got container create event")
	// Schedule container on every node
	for nodeName, stream := range n.streams {

		// Extract the Container from the Event
		var ctr containers.Container
		if err := e.GetObject().UnmarshalTo(&ctr); err != nil {
			return err
		}

		// Lookup the Node
		res, err := n.Get(context.Background(), &nodes.GetNodeRequest{Id: nodeName})
		if err != nil {
			return err
		}

		node := res.GetNode()

		// See if nodeSelector matches labels on the node
		nodeSelector := labels.NewCompositeSelectorFromMap(ctr.GetConfig().GetNodeSelector())
		if !nodeSelector.Matches(node.GetMeta().GetLabels()) {
			return nil
		}

		// Schedule container on node
		n.logger.Info("scheduling container", "node", nodeName, "container", ctr.GetMeta().GetName())
		err = stream.Send(e)
		if err != nil {
			return err
		}
	}

	return nil
}

func (n *NodeService) onContainerDelete(e *eventsv1.Event) error {
	n.logger.Info("onContainerDelete in service")
	// Unmarshal Container from event
	var ctr containers.Container
	err := e.GetObject().UnmarshalTo(&ctr)
	if err != nil {
		return err
	}

	// Figure out which node the container is running on
	nodeName := ctr.GetStatus().GetNode()
	if nodeName == "" {
		return fmt.Errorf("container is missing node in status")
	}

	stream, ok := n.streams[nodeName]
	if !ok {
		return fmt.Errorf("node is not connected as %s", nodeName)
	}

	// Forward event to node
	err = stream.Send(e)
	if err != nil {
		return err
	}

	return nil
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
