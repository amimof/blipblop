package controller

import (
	"context"
	"os"
	"time"

	eventsv1 "github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/amimof/blipblop/pkg/events"
	"github.com/amimof/blipblop/pkg/logger"
	"github.com/amimof/blipblop/pkg/runtime"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/metadata"
)

type NodeController struct {
	clientset                *client.ClientSet
	runtime                  runtime.Runtime
	handlers                 *NodeEventHandlerFuncs
	logger                   logger.Logger
	connectionState          connectivity.State
	heartbeatIntervalSeconds int
}

type NodeEventHandlerFuncs struct {
	OnNodeDelete events.EventHandlerFunc
	OnNodeJoin   events.EventHandlerFunc
	OnNodeForget events.EventHandlerFunc
}

type NewNodeControllerOption func(c *NodeController)

func WithNodeControllerLogger(l logger.Logger) NewNodeControllerOption {
	return func(c *NodeController) {
		c.logger = l
	}
}

func WithHeartbeatInterval(s int) NewNodeControllerOption {
	return func(c *NodeController) {
		c.heartbeatIntervalSeconds = s
	}
}

func (c *NodeController) heartBeat(ctx context.Context) {
	hostname, _ := os.Hostname()
	for {
		state := c.clientset.State()

		c.connectionState = state
		err := c.clientset.NodeV1().SetState(ctx, hostname, state)
		if err != nil {
			c.logger.Error("error setting node state", "error", err, "node", hostname, "state", state)
		}

		if state == connectivity.Shutdown {
			c.logger.Info("Connection shutdown detected")
			break
		}

		time.Sleep(time.Second * time.Duration(c.heartbeatIntervalSeconds))
	}
}

func (c *NodeController) Run(ctx context.Context, stopCh <-chan struct{}) {
	// Setup channels
	evt := make(chan *eventsv1.Event, 10)
	errChan := make(chan error, 10)

	// Setup handlers
	handlers := events.NodeEventHandlerFuncs{
		OnCreate: c.onNodeCreate,
		OnUpdate: c.onNodeUpdate,
		OnDelete: c.onNodeDelete,
		OnJoin:   c.onNodeJoin,
		OnForget: c.onNodeForget,
	}

	// Run informer
	informer := events.NewNodeEventInformer(handlers)
	go informer.Run(evt)

	// Run heart beat routine
	go c.heartBeat(ctx)

	// Attach node name to context
	hostname, err := os.Hostname()
	if err != nil {
		c.logger.Error("error getting hostname", "error", err)
	}
	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_node_name", hostname)

	// Subscribe with retry
	for {
		select {
		case <-stopCh:
			c.logger.Info("done watching, stopping subscription")
			return
		default:
			if err := c.clientset.EventV1().Subscribe(ctx, evt, errChan); err != nil {
				c.logger.Error("error occured during subscribe", "error", err)
			}

			c.logger.Info("attempting to re-subscribe to event server")
			time.Sleep(5 * time.Second)
		}
	}
}

func (c *NodeController) onNodeCreate(_ *eventsv1.Event) error {
	return nil
}

func (c *NodeController) onNodeUpdate(_ *eventsv1.Event) error {
	return nil
}

// BUG: this will delete and forget the node that the event was triggered on. We need to make sure
// that the node that receives the event, checks that the id matches the node id so that only
// the specific node unregisters itself from the server
func (c *NodeController) onNodeDelete(obj *eventsv1.Event) error {
	ctx := context.Background()
	err := c.clientset.NodeV1().Forget(ctx, obj.GetMeta().GetName())
	if err != nil {
		c.logger.Error("error unjoining node", "node", obj.GetMeta().GetName(), "error", err)
		return err
	}
	c.logger.Debug("successfully unjoined node", "node", obj.GetMeta().GetName())
	return nil
}

func (c *NodeController) onNodeJoin(obj *eventsv1.Event) error {
	return nil
}

func (c *NodeController) onNodeForget(obj *eventsv1.Event) error {
	return nil
}

func (c *NodeController) handleEvent(ev *eventsv1.Event) error {
	if ev == nil {
		return nil
	}
	t := ev.Type
	switch t {
	case eventsv1.EventType_NodeDelete:
		return c.handlers.OnNodeDelete(ev)
	case eventsv1.EventType_NodeJoin:
		return c.handlers.OnNodeJoin(ev)
	case eventsv1.EventType_NodeForget:
		return c.handlers.OnNodeForget(ev)
	default:
		c.logger.Debug("node handler not implemented for event", "type", t.String())
	}
	return nil
}

// Reconcile ensures that desired containers matches with containers
// in the runtime environment. It removes any containers that are not
// desired (missing from the server) and adds those missing from runtime.
// It is preferrably run early during startup of the controller.
func (n *NodeController) Reconcile(ctx context.Context) error {
	return nil
}

func NewNodeController(c *client.ClientSet, rt runtime.Runtime, opts ...NewNodeControllerOption) *NodeController {
	m := &NodeController{
		clientset:                c,
		runtime:                  rt,
		logger:                   logger.ConsoleLogger{},
		heartbeatIntervalSeconds: 5,
	}
	m.handlers = &NodeEventHandlerFuncs{
		OnNodeDelete: m.onNodeDelete,
		OnNodeJoin:   m.onNodeJoin,
		OnNodeForget: m.onNodeForget,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}
