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
	"github.com/google/uuid"
	"google.golang.org/grpc/connectivity"
)

type NodeController struct {
	clientset                *client.ClientSet
	runtime                  runtime.Runtime
	handlers                 *NodeEventHandlerFuncs
	logger                   logger.Logger
	connectionState          connectivity.State
	heartbeatIntervalSeconds int
	nodeName                 string
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

func WithNodeName(s string) NewNodeControllerOption {
	return func(c *NodeController) {
		c.nodeName = s
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

		err = c.clientset.NodeV1().SendMessage(ctx, &eventsv1.Event{Type: eventsv1.EventType_NodeJoin})
		if err != nil {
			c.logger.Error("error sending message", "error", err)
		}

		time.Sleep(time.Second * time.Duration(c.heartbeatIntervalSeconds))
	}
}

// func (c *NodeController) Run(ctx context.Context, stopCh <-chan struct{}) {
func (c *NodeController) Run(ctx context.Context) {
	// Setup channels
	evt := make(chan *eventsv1.Event, 10)
	errChan := make(chan error, 1)

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

	// Connect with retry logic
	go func() {
		err := c.clientset.NodeV1().Connect(ctx, c.nodeName, evt, errChan)
		if err != nil {
			c.logger.Error("error connecting to server", "error", err)
		}
	}()

	// Handle messages
	for {
		select {
		case err := <-errChan:
			c.logger.Error("received stream error", "error", err)
		case <-ctx.Done():
			c.logger.Info("context canceled, shutting down controller")
			return
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
	c.logger.Info("node joined")
	return nil
}

func (c *NodeController) onNodeForget(obj *eventsv1.Event) error {
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
		nodeName:                 uuid.New().String(),
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
