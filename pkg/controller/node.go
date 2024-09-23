package controller

import (
	"context"
	"log"
	"time"

	"github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/amimof/blipblop/pkg/logger"
	"github.com/amimof/blipblop/pkg/runtime"
)

type NodeController struct {
	clientset *client.ClientSet
	runtime   runtime.Runtime
	handlers  *NodeEventHandlerFuncs
	logger    logger.Logger
}
type NodeEventHandlerFuncs struct {
	OnNodeDelete func(obj *events.Event)
}

type NewNodeControllerOption func(c *NodeController)

func WithNodeControllerLogger(l logger.Logger) NewNodeControllerOption {
	return func(c *NodeController) {
		c.logger = l
	}
}

func (c *NodeController) Run(ctx context.Context, stopCh <-chan struct{}) {
	// Setup channels
	evt := make(chan *events.Event)
	errChan := make(chan error)

	go func() {
		for {
			select {
			case ev := <-evt:
				c.logger.Debug("node controller received event", "id", ev.GetId(), "type", ev.GetType().String())
				c.handleEvent(ev)
			case err := <-errChan:
				c.logger.Error("node controller recevied error on channel", "error", err)
				return
			case <-stopCh:
				c.logger.Info("done watching, closing node controller")
				ctx.Done()
				return
			}
		}
	}()

	for {
		if err := c.clientset.EventV1().Subscribe(ctx, evt, errChan); err != nil {
			c.logger.Error("error occured during subscribe", "error", err)
		}

		c.logger.Info("attempting to re-subscribe to event server")
		time.Sleep(5 * time.Second)

	}
}

// BUG: this will delete and forget the node that the event was triggered on. We need to make sure
// that the node that receives the event, checks that the id matches the node id so that only
// the specific node unregisters itself from the server
func (c *NodeController) onNodeDelete(obj *events.Event) {
	ctx := context.Background()
	err := c.clientset.NodeV1().ForgetNode(ctx, obj.GetId())
	if err != nil {
		log.Printf("error unjoining node %s: %v", obj.GetId(), err)
		return
	}
	log.Printf("successfully unjoined node %s", obj.GetId())
}

func (c *NodeController) handleEvent(ev *events.Event) {
	if ev == nil {
		return
	}
	t := ev.Type
	switch t {
	case events.EventType_NodeDelete:
		c.handlers.OnNodeDelete(ev)
	default:
		c.logger.Warn("Node handler not implemented for event", "type", t.String())
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
		clientset: c,
		runtime:   rt,
		logger:    logger.ConsoleLogger{},
	}
	m.handlers = &NodeEventHandlerFuncs{
		OnNodeDelete: m.onNodeDelete,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}
