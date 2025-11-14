package controller

import (
	"context"
	"os"

	containersv1 "github.com/amimof/blipblop/api/services/containers/v1"
	eventsv1 "github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/api/services/nodes/v1"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/amimof/blipblop/pkg/errors"
	"github.com/amimof/blipblop/pkg/events"
	"github.com/amimof/blipblop/pkg/logger"
	"github.com/amimof/blipblop/pkg/runtime"
	"github.com/google/uuid"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type NodeController struct {
	runtime                  runtime.Runtime
	logger                   logger.Logger
	clientset                *client.ClientSet
	nodeName                 string
	heartbeatIntervalSeconds int
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

// Run implements controller
func (c *NodeController) Run(ctx context.Context) {
	// Subscribe to events
	evt, errCh := c.clientset.EventV1().Subscribe(ctx, events.ALL...)

	// Setup Node Handlers
	c.clientset.EventV1().On(events.NodeCreate, c.onNodeCreate)
	c.clientset.EventV1().On(events.NodeUpdate, c.onNodeUpdate)
	c.clientset.EventV1().On(events.NodeDelete, c.onNodeDelete)
	c.clientset.EventV1().On(events.NodeJoin, c.onNodeJoin)
	c.clientset.EventV1().On(events.NodeForget, c.onNodeForget)

	// Setup container handlers
	// c.clientset.EventV1().On(events.ContainerCreate, c.onContainerCreate)
	c.clientset.EventV1().On(events.ContainerDelete, c.handleErrors(c.onContainerDelete))
	c.clientset.EventV1().On(events.ContainerUpdate, c.handleErrors(c.onContainerUpdate))
	c.clientset.EventV1().On(events.ContainerStop, c.handleErrors(c.onContainerStop))
	c.clientset.EventV1().On(events.ContainerKill, c.handleErrors(c.onContainerKill))
	c.clientset.EventV1().On(events.ContainerStart, c.handleErrors(c.onContainerStart))
	c.clientset.EventV1().On(events.Schedule, c.handleErrors(c.onSchedule))

	go func() {
		for e := range evt {
			c.logger.Info("Got event", "event", e.GetType().String())
		}
	}()

	// Connect with retry logic
	go func() {
		err := c.clientset.NodeV1().Connect(ctx, c.nodeName, evt, errCh)
		if err != nil {
			c.logger.Error("error connecting to server", "error", err)
		}
	}()

	// Get hostname from environment
	hostname, err := os.Hostname()
	if err != nil {
		c.logger.Error("error retrieving hostname from environment", "error", err)
	}

	// Get version from runtime
	runtimeVer, err := c.runtime.Version(ctx)
	if err != nil {
		c.logger.Error("error retrieving version from runtime", "error", err)
	}

	// Update status once connected
	err = c.clientset.NodeV1().Status().Update(
		ctx,
		c.nodeName, &nodes.Status{
			Phase:    wrapperspb.String(connectivity.Ready.String()),
			Hostname: wrapperspb.String(hostname),
			Runtime:  wrapperspb.String(runtimeVer),
		},
		"phase",
		"hostname",
		"runtime",
	)
	if err != nil {
		c.logger.Error("error setting node state", "error", err)
	}

	// Handle errors
	for e := range errCh {
		c.logger.Error("received error on channel", "error", e)
	}
}

func (c *NodeController) handleErrors(h events.HandlerFunc) events.HandlerFunc {
	return func(ctx context.Context, ev *eventsv1.Event) error {
		err := h(ctx, ev)
		if err != nil {
			c.logger.Error("handler returned error", "event", ev.GetType().String(), "error", err)
			return err
		}
		return nil
	}
}

func (c *NodeController) onSchedule(ctx context.Context, obj *eventsv1.Event) error {
	// Unwrap schedule type from the event
	s := &eventsv1.ScheduleRequest{}
	err := obj.GetObject().UnmarshalTo(s)
	if err != nil {
		return err
	}

	// Unwrap the node from the event object
	n := &nodes.Node{}
	err = s.GetNode().UnmarshalTo(n)
	if err != nil {
		return err
	}

	// We only care if the event is for us
	if n.GetMeta().GetName() != c.nodeName {
		return nil
	}

	var ctr containersv1.Container
	err = s.GetContainer().UnmarshalTo(&ctr)
	if err != nil {
		return err
	}

	newObj := proto.Clone(obj).(*eventsv1.Event)
	newObj.Object, err = anypb.New(&ctr)
	if err != nil {
		return err
	}

	err = c.onContainerStart(ctx, newObj)
	if err != nil {
		return err
	}

	return nil
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

// When this is run we can assume that the container has been scheduled
func (c *NodeController) onContainerCreate(ctx context.Context, e *eventsv1.Event) error {
	return nil
}

func (c *NodeController) onContainerDelete(ctx context.Context, e *eventsv1.Event) error {
	var ctr containersv1.Container
	err := e.GetObject().UnmarshalTo(&ctr)
	if err != nil {
		return err
	}

	containerID := ctr.GetMeta().GetName()
	c.logger.Info("controller received task", "event", e.GetType().String(), "name", ctr.GetMeta().GetName())

	err = c.onContainerStop(ctx, e)
	if err != nil {
		return err
	}

	err = c.onContainerStop(ctx, e)
	if err != nil {
		return err
	}

	err = c.runtime.Delete(ctx, &ctr)
	if err != nil {
		_ = c.clientset.ContainerV1().Status().Update(
			ctx,
			containerID,
			&containersv1.Status{
				Phase:  wrapperspb.String(errors.ERRDELETE),
				Status: wrapperspb.String(err.Error()),
			}, "phase", "status")
		return err
	}
	return nil
}

func (c *NodeController) onContainerUpdate(ctx context.Context, e *eventsv1.Event) error {
	err := c.onContainerStop(ctx, e)
	if err != nil {
		return err
	}
	err = c.onContainerStart(ctx, e)
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

	containerID := ctr.GetMeta().GetName()
	c.logger.Info("controller received task", "event", e.GetType().String(), "name", containerID)

	_ = c.clientset.ContainerV1().Status().Update(ctx, containerID, &containersv1.Status{Phase: wrapperspb.String("stopping")}, "phase")
	err = c.runtime.Kill(ctx, &ctr)
	if err != nil {
		_ = c.clientset.ContainerV1().Status().Update(
			ctx,
			containerID,
			&containersv1.Status{
				Phase:  wrapperspb.String(errors.ERRKILL),
				Status: wrapperspb.String(err.Error()),
			}, "phase", "status")
		return err
	}

	err = c.onContainerDelete(ctx, e)
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

	containerID := ctr.GetMeta().GetName()
	c.logger.Debug("controller received task", "event", e.GetType().String(), "name", containerID)

	// Pull image
	_ = c.clientset.ContainerV1().Status().Update(ctx, containerID, &containersv1.Status{Phase: wrapperspb.String("pulling")}, "phase")
	err = c.runtime.Pull(ctx, &ctr)
	if err != nil {
		_ = c.clientset.ContainerV1().Status().Update(
			ctx,
			containerID,
			&containersv1.Status{
				Phase:  wrapperspb.String(errors.ERRIMAGEPULL),
				Status: wrapperspb.String(err.Error()),
			}, "phase", "status")
		return err
	}

	// Run container
	err = c.runtime.Run(ctx, &ctr)
	if err != nil {
		_ = c.clientset.ContainerV1().Status().Update(
			ctx,
			containerID,
			&containersv1.Status{
				Phase:  wrapperspb.String(errors.ERREXEC),
				Status: wrapperspb.String(err.Error()),
			}, "phase", "status")
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

	containerID := ctr.GetMeta().GetName()
	c.logger.Info("controller received task", "event", e.GetType().String(), "name", containerID)

	_ = c.clientset.ContainerV1().Status().Update(ctx, containerID, &containersv1.Status{Phase: wrapperspb.String("stopping")}, "phase")
	err = c.runtime.Stop(ctx, &ctr)
	if err != nil {
		_ = c.clientset.ContainerV1().Status().Update(
			ctx,
			containerID,
			&containersv1.Status{
				Phase:  wrapperspb.String(errors.ERRSTOP),
				Status: wrapperspb.String(err.Error()),
			}, "phase", "status")
		return err
	}

	_ = c.clientset.ContainerV1().Status().Update(ctx, containerID, &containersv1.Status{Phase: wrapperspb.String("deleting")}, "phase")
	err = c.onContainerDelete(ctx, e)
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

	return m, nil
}
