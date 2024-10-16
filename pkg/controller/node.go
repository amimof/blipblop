package controller

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/amimof/blipblop/pkg/logger"
	"github.com/amimof/blipblop/pkg/runtime"
	"google.golang.org/grpc/connectivity"
)

type NodeController struct {
	clientset                *client.ClientSet
	runtime                  runtime.Runtime
	handlers                 *NodeEventHandlerFuncs
	logger                   logger.Logger
	connectionState          connectivity.State
	heartbeatIntervalSeconds int
}

type NodeEventHandlerFunc func(obj *events.Event)

type NodeEventHandlerFuncs struct {
	OnNodeDelete NodeEventHandlerFunc
	OnNodeJoin   NodeEventHandlerFunc
	OnNodeForget NodeEventHandlerFunc
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
	evt := make(chan *events.Event, 10)
	errChan := make(chan error, 10)
	go func() {
		for {
			select {
			case ev := <-evt:
				c.logger.Debug("node controller received event", "id", ev.GetMeta().GetName(), "type", ev.GetType().String())
				c.handleEvent(ev)
				continue
			case err := <-errChan:
				c.logger.Error("node controller recevied error on channel", "error", err)
				return
			case <-stopCh:
				c.logger.Info("done watching, closing node controller")
				return
			}
		}
	}()

	// Run heart beat routine
	go c.heartBeat(ctx)

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

// BUG: this will delete and forget the node that the event was triggered on. We need to make sure
// that the node that receives the event, checks that the id matches the node id so that only
// the specific node unregisters itself from the server
func (c *NodeController) onNodeDelete(obj *events.Event) {
	ctx := context.Background()
	err := c.clientset.NodeV1().Forget(ctx, obj.GetMeta().GetName())
	if err != nil {
		log.Printf("error unjoining node %s: %v", obj.GetMeta().GetName(), err)
		return
	}
	log.Printf("successfully unjoined node %s", obj.GetMeta().GetName())
}

func (c *NodeController) onNodeJoin(obj *events.Event) {
}

func (c *NodeController) onNodeForget(obj *events.Event) {
}

func (c *NodeController) handleEvent(ev *events.Event) {
	if ev == nil {
		return
	}
	t := ev.Type
	switch t {
	case events.EventType_NodeDelete:
		c.handlers.OnNodeDelete(ev)
	case events.EventType_NodeJoin:
		c.handlers.OnNodeJoin(ev)
	case events.EventType_NodeForget:
		c.handlers.OnNodeForget(ev)
	default:
		c.logger.Debug("node handler not implemented for event", "type", t.String())
	}
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
