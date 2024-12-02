package controller

import (
	"context"

	containersv1 "github.com/amimof/blipblop/api/services/containers/v1"
	eventsv1 "github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/amimof/blipblop/pkg/events/informer"
	"github.com/amimof/blipblop/pkg/logger"
	"github.com/amimof/blipblop/pkg/runtime"
	"github.com/google/uuid"
	"google.golang.org/grpc/connectivity"
)

type NodeController struct {
	clientset                *client.ClientSet
	runtime                  runtime.Runtime
	logger                   logger.Logger
	heartbeatIntervalSeconds int
	nodeName                 string
}

type NodeEventHandlerFuncs struct {
	OnNodeDelete informer.EventHandlerFunc
	OnNodeJoin   informer.EventHandlerFunc
	OnNodeForget informer.EventHandlerFunc
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

// func (c *NodeController) Run(ctx context.Context, stopCh <-chan struct{}) {
func (c *NodeController) Run(ctx context.Context) {
	// Setup channels
	evt := make(chan *eventsv1.Event, 10)
	errChan := make(chan error, 1)

	nodeEvt := make(chan *eventsv1.Event, 10)
	containerEvt := make(chan *eventsv1.Event, 10)

	// Setup node handlers
	nodeHandlers := informer.NodeEventHandlerFuncs{
		OnCreate: c.onNodeCreate,
		OnUpdate: c.onNodeUpdate,
		OnDelete: c.onNodeDelete,
		OnJoin:   c.onNodeJoin,
		OnForget: c.onNodeForget,
	}

	// Setup container handlers
	containerHandlers := informer.ContainerEventHandlerFuncs{
		OnCreate: c.onContainerCreate,
		OnDelete: c.onContainerDelete,
		OnUpdate: c.onContainerUpdate,
		OnStop:   c.onContainerStop,
		OnKill:   c.onContainerKill,
		OnStart:  c.onContainerStart,
	}

	// Run node informer
	nodeInformer := informer.NewNodeEventInformer(nodeHandlers)
	go nodeInformer.Run(ctx, nodeEvt)

	// Run container informer
	containerInformer := informer.NewContainerEventInformer(containerHandlers, informer.WithContainerEventInformerLogger(c.logger))
	go containerInformer.Run(ctx, containerEvt)

	// Start broadcasting incoming events to both node and container informers
	go c.broadcastEvent(ctx, evt, nodeEvt, containerEvt)

	// Connect with retry logic
	go func() {
		err := c.clientset.NodeV1().Connect(ctx, c.nodeName, evt, errChan)
		if err != nil {
			c.logger.Error("error connecting to server", "error", err)
		}
	}()

	// Update status once connected
	err := c.clientset.NodeV1().SetState(ctx, c.nodeName, connectivity.Ready)
	if err != nil {
		c.logger.Error("error setting node state", "error", err)
	}

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

// Broadcaster reads from the input channel and sends each message to multiple output channels.
func (n *NodeController) broadcastEvent(_ context.Context, input <-chan *eventsv1.Event, outputs ...chan *eventsv1.Event) {
	// Send the same message to each output channel
	for event := range input {
		for _, out := range outputs {
			// event.Meta.Labels = metadata
			out <- event
		}
	}

	// Close all output channels
	for _, out := range outputs {
		close(out)
	}
}

func (c *NodeController) onNodeCreate(ctx context.Context, _ *eventsv1.Event) error {
	return nil
}

func (c *NodeController) onNodeUpdate(ctx context.Context, _ *eventsv1.Event) error {
	return nil
}

// BUG: this will delete and forget the node that the event was triggered on. We need to make sure
// that the node that receives the event, checks that the id matches the node id so that only
// the specific node unregisters itself from the server
func (c *NodeController) onNodeDelete(ctx context.Context, obj *eventsv1.Event) error {
	err := c.clientset.NodeV1().Forget(ctx, obj.GetMeta().GetName())
	if err != nil {
		c.logger.Error("error unjoining node", "node", obj.GetMeta().GetName(), "error", err)
		return err
	}
	c.logger.Debug("successfully unjoined node", "node", obj.GetMeta().GetName())
	return nil
}

func (c *NodeController) onNodeJoin(ctx context.Context, obj *eventsv1.Event) error {
	return nil
}

func (c *NodeController) onNodeForget(ctx context.Context, obj *eventsv1.Event) error {
	return nil
}

func (c *NodeController) onContainerCreate(ctx context.Context, e *eventsv1.Event) error {
	// Get the container
	var ctr containersv1.Container
	err := e.Object.UnmarshalTo(&ctr)
	if err != nil {
		return err
	}

	c.logger.Info("controller received task", "event", e.GetType().String(), "name", ctr.GetMeta().GetName())

	err = c.runtime.Pull(ctx, &ctr)
	if err != nil {
		return err
	}
	err = c.runtime.Run(ctx, &ctr)
	if err != nil {
		return err
	}

	return nil
}

func (c *NodeController) onContainerDelete(ctx context.Context, e *eventsv1.Event) error {
	var ctr containersv1.Container
	err := e.GetObject().UnmarshalTo(&ctr)
	if err != nil {
		return err
	}

	c.logger.Info("controller received task", "event", e.GetType().String(), "name", ctr.GetMeta().GetName())

	err = c.runtime.Kill(ctx, &ctr)
	if err != nil {
		return err
	}
	err = c.runtime.Delete(ctx, &ctr)
	if err != nil {
		return err
	}
	return nil
}

func (c *NodeController) onContainerUpdate(ctx context.Context, e *eventsv1.Event) error {
	var ctr containersv1.Container
	err := e.GetObject().UnmarshalTo(&ctr)
	if err != nil {
		return err
	}

	c.logger.Info("controller received task", "event", e.GetType().String(), "name", ctr.GetMeta().GetName())

	err = c.runtime.Kill(ctx, &ctr)
	if err != nil {
		return err
	}
	err = c.runtime.Delete(ctx, &ctr)
	if err != nil {
		return err
	}
	err = c.runtime.Pull(ctx, &ctr)
	if err != nil {
		return err
	}
	err = c.runtime.Run(ctx, &ctr)
	if err != nil {
		return err
	}
	return nil
}

func (c *NodeController) onContainerKill(ctx context.Context, e *eventsv1.Event) error {
	var ctr containersv1.Container
	err := e.GetObject().UnmarshalTo(&ctr)
	if err != nil {
		return err
	}

	c.logger.Info("controller received task", "event", e.GetType().String(), "name", ctr.GetMeta().GetName())

	err = c.runtime.Kill(ctx, &ctr)
	if err != nil {
		return err
	}

	return nil
}

func (c *NodeController) onContainerStart(ctx context.Context, e *eventsv1.Event) error {
	var ctr containersv1.Container
	err := e.GetObject().UnmarshalTo(&ctr)
	if err != nil {
		return err
	}

	c.logger.Info("controller received task", "event", e.GetType().String(), "name", ctr.GetMeta().GetName())

	// Delete container if it exists
	_ = c.clientset.ContainerV1().SetTaskStatus(ctx, ctr.GetMeta().GetName(), containersv1.Phase_Stopping.String(), "")
	if err := c.runtime.Delete(ctx, &ctr); err != nil {
		return err
	}

	// Pull image
	_ = c.clientset.ContainerV1().SetTaskStatus(ctx, ctr.GetMeta().GetName(), containersv1.Phase_Pulling.String(), "")
	err = c.runtime.Pull(ctx, &ctr)
	if err != nil {
		return err
	}

	// Run container
	_ = c.clientset.ContainerV1().SetTaskStatus(ctx, ctr.GetMeta().GetName(), containersv1.Phase_Starting.String(), "")
	err = c.runtime.Run(ctx, &ctr)
	if err != nil {
		return err
	}

	return nil
}

func (c *NodeController) onContainerStop(ctx context.Context, e *eventsv1.Event) error {
	var ctr containersv1.Container
	err := e.GetObject().UnmarshalTo(&ctr)
	if err != nil {
		return err
	}

	c.logger.Info("controller received task", "event", e.GetType().String(), "name", ctr.GetMeta().GetName())

	err = c.runtime.Stop(ctx, &ctr)
	if err != nil {
		return err
	}

	return nil
}

// Reconcile ensures that desired containers matches with containers
// in the runtime environment. It removes any containers that are not
// desired (missing from the server) and adds those missing from runtime.
// It is preferrably run early during startup of the controller.
func (c *NodeController) Reconcile(ctx context.Context) error {
	return nil
}

func NewNodeController(c *client.ClientSet, rt runtime.Runtime, opts ...NewNodeControllerOption) (*NodeController, error) {
	m := &NodeController{
		clientset:                c,
		runtime:                  rt,
		logger:                   logger.ConsoleLogger{},
		heartbeatIntervalSeconds: 5,
		nodeName:                 uuid.New().String(),
	}
	for _, opt := range opts {
		opt(m)
	}

	// Setup tracing
	// exporter, err := otlptracehttp.New(context.Background(), otlptracehttp.WithInsecure(), otlptracehttp.WithEndpoint("192.168.13.123:4318"))
	// if err != nil {
	// 	return nil, err
	// }
	// defaultResource := resource.Default()
	//
	// mergedResources, err := resource.Merge(
	// 	defaultResource,
	// 	resource.NewWithAttributes(
	// 		defaultResource.SchemaURL(),
	// 		semconv.ServiceNameKey.String("blipblop-node"),
	// 	),
	// )
	// if err != nil {
	// 	return nil, err
	// }
	//
	// tracer := sdktrace.NewTracerProvider(
	// 	sdktrace.WithBatcher(exporter),
	// 	sdktrace.WithSampler(sdktrace.AlwaysSample()),
	// 	sdktrace.WithResource(mergedResources),
	// )
	//
	// otel.SetTracerProvider(tracer)
	//
	return m, nil
}
