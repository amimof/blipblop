package controller

import (
	"bufio"
	"context"
	"os"
	"sync"
	"time"

	containersv1 "github.com/amimof/blipblop/api/services/containers/v1"
	eventsv1 "github.com/amimof/blipblop/api/services/events/v1"
	logsv1 "github.com/amimof/blipblop/api/services/logs/v1"
	"github.com/amimof/blipblop/api/services/nodes/v1"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/amimof/blipblop/pkg/consts"
	"github.com/amimof/blipblop/pkg/errors"
	"github.com/amimof/blipblop/pkg/events"
	"github.com/amimof/blipblop/pkg/logger"
	"github.com/amimof/blipblop/pkg/runtime"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type NodeController struct {
	runtime                  runtime.Runtime
	logger                   logger.Logger
	clientset                *client.ClientSet
	nodeName                 string
	heartbeatIntervalSeconds int
	tracer                   trace.Tracer
	logChan                  chan *logsv1.LogEntry
	activeLogStreams         map[events.LogKey]context.CancelFunc
	logStreamsMu             sync.Mutex
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
	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_controller_name", "node")
	evt, errCh := c.clientset.EventV1().Subscribe(ctx, events.ALL...)

	// Setup Node Handlers
	c.clientset.EventV1().On(events.NodeCreate, c.onNodeCreate)
	c.clientset.EventV1().On(events.NodeUpdate, c.handleErrors(c.onNodeUpdate))
	c.clientset.EventV1().On(events.NodeDelete, c.onNodeDelete)
	c.clientset.EventV1().On(events.NodeJoin, c.onNodeJoin)
	c.clientset.EventV1().On(events.NodeForget, c.onNodeForget)
	c.clientset.EventV1().On(events.NodeConnect, c.onNodeConnect)

	// Setup container handlers
	c.clientset.EventV1().On(events.ContainerDelete, c.handleErrors(c.onContainerDelete))
	c.clientset.EventV1().On(events.ContainerUpdate, c.handleErrors(c.onContainerUpdate))
	c.clientset.EventV1().On(events.ContainerStop, c.handleErrors(c.onContainerStop))
	c.clientset.EventV1().On(events.ContainerKill, c.handleErrors(c.onContainerKill))
	c.clientset.EventV1().On(events.ContainerStart, c.handleErrors(c.onContainerStart))
	c.clientset.EventV1().On(events.Schedule, c.handleErrors(c.onSchedule))

	// Setup log handlers
	c.clientset.EventV1().On(events.TailLogsStart, c.handleErrors(c.onLogStart))
	c.clientset.EventV1().On(events.TailLogsStop, c.handleErrors(c.onLogStop))

	go func() {
		for e := range evt {
			c.logger.Info("Got event", "event", e.GetType().String(), "clientID", c.nodeName, "objectID", e.GetObjectId())
		}
	}()

	// Connect with retry logic
	connErr := make(chan error, 1)
	go func() {
		err := c.clientset.NodeV1().Connect(ctx, c.nodeName, evt, connErr)
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
			Phase:    wrapperspb.String(consts.PHASEREADY),
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
	for {
		select {
		case <-ctx.Done():
			return
		case e, ok := <-errCh:
			if !ok {
				errCh = nil
				continue
			}
			if e != nil {
				c.logger.Error("received error on channel", "error", e)
			}
		case e, ok := <-connErr:
			if !ok {
				connErr = nil
				continue
			}
			if e != nil {
				c.logger.Error("received error on channel", "error", e)
			}
		}
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

func (c *NodeController) onLogStart(ctx context.Context, obj *eventsv1.Event) error {
	s := &logsv1.TailLogRequest{}
	err := obj.GetObject().UnmarshalTo(s)
	if err != nil {
		return err
	}
	c.logger.Debug("someone requested logs", "nodeID", s.GetNodeId(), "containerID", s.GetContainerId(), "sessionID", s.GetSessionId())

	if s.GetNodeId() != c.nodeName {
		return nil
	}

	streamKey := events.LogKey{
		NodeID:      s.GetNodeId(),
		ContainerID: s.GetContainerId(),
		SessionID:   s.GetSessionId(),
	}

	c.logStreamsMu.Lock()
	if _, exists := c.activeLogStreams[streamKey]; exists {
		c.logStreamsMu.Unlock()
		c.logger.Debug("log stream already active", "nodeID", s.GetNodeId(), "containerID", s.GetContainerId(), "sessionID", s.GetSessionId())
		return nil
	}
	c.logStreamsMu.Unlock()

	containerIO, err := c.runtime.IO(ctx, s.GetContainerId())
	if err != nil {
		c.logger.Error("error getting container logs", "error", err)
		return err
	}

	logStream, err := c.clientset.LogV1().Stream(ctx)
	if err != nil {
		c.logger.Error("error setting up pushlogs", "error", err)
		return err
	}

	streamCtx, cancel := context.WithCancel(ctx)

	c.logStreamsMu.Lock()
	c.activeLogStreams[streamKey] = cancel
	c.logStreamsMu.Unlock()

	go func() {
		c.logger.Info("starting log scanner goroutine", "nodeID", s.GetNodeId(), "containerID", s.GetContainerId())

		defer func() {
			c.logStreamsMu.Lock()
			delete(c.activeLogStreams, streamKey)
			c.logStreamsMu.Unlock()
			cancel()
			_ = logStream.Close()
			if containerIO.Stdout != nil {
				_ = containerIO.Stdout.Close()
			}
		}()

		// Setup scanner. We use a channel to send each line through
		lines := make(chan string)
		scanner := bufio.NewScanner(containerIO.Stdout)

		// Goroutine that scans the log file and sends each line on the channel
		go func() {
			defer close(lines)

			for {

				// Exit early on cancel even when at EOF
				select {
				case <-streamCtx.Done():
					c.logger.Debug("exiting log streaming because context was cancelled", "nodeID", s.GetNodeId(), "containerID", s.GetContainerId())
					return
				default:
				}

				// Scan log file and send each line through the channel
				if scanner.Scan() {
					line := scanner.Text()
					select {
					case <-streamCtx.Done():
						c.logger.Debug("exiting log streaming because context was cancelled", "nodeID", s.GetNodeId(), "containerID", s.GetContainerId())
						return
					case lines <- line: // Send line through
					}
					continue
				}

				// Scanner returned false. Check for errors and exit out if any
				if err := scanner.Err(); err != nil {
					c.logger.Error(
						"error reading from stdout",
						"error", err,
						"nodeID", s.GetNodeId(),
						"containerID", s.GetContainerId(),
					)
					return
				}

				// No errors, maybe EOF?
				select {
				case <-streamCtx.Done():
					c.logger.Debug("exiting log streaming because context was cancelled", "nodeID", s.GetNodeId(), "containerID", s.GetContainerId())
					return
				case <-time.After(300 * time.Millisecond): // Wait a bit before iterating again
				}

				// EOF reached, ovewrite the scanner to start reading again
				scanner = bufio.NewScanner(containerIO.Stdout)
			}
		}()

		// Count the number of lines read
		var seq uint64

		// Send log entry for each line that comes in from the line channel. After x amount of time anf if no lines are
		// received, exit out. This is a blocking operation.
		for {
			select {
			case <-streamCtx.Done():
				c.logger.Debug("log stream cancelled", "nodeID", s.GetNodeId(), "containerID", s.GetContainerId())
				return
			case <-time.After(5 * time.Minute):
				c.logger.Debug("scanner timeout - no data received", "nodeID", s.GetNodeId(), "containerID", s.GetContainerId())
				return
			case line, ok := <-lines:

				if !ok {
					c.logger.Debug("log stream completed", "nodeID", s.GetNodeId(), "containerID", s.GetContainerId())
					return
				}

				// Send the line as log entry to the server
				if err := logStream.Send(&logsv1.LogEntry{
					ContainerId: s.GetContainerId(),
					NodeId:      s.GetNodeId(),
					SessionId:   s.GetSessionId(),
					Timestamp:   timestamppb.Now(),
					Line:        line,
					Seq:         seq,
				}); err != nil {
					c.logger.Error(
						"error pushing log entry",
						"error", err,
						"containerID", s.GetContainerId(),
						"nodeID", s.GetNodeId(),
						"sessionID", s.GetSessionId(),
						"seq", seq,
					)
					return
				}

				// Increase counter
				seq += 1
			}
		}
	}()

	return nil
}

func (c *NodeController) onLogStop(ctx context.Context, obj *eventsv1.Event) error {
	s := &logsv1.TailLogRequest{}
	err := obj.GetObject().UnmarshalTo(s)
	if err != nil {
		return err
	}
	c.logger.Debug("someone requested stop logs", "nodeID", s.GetNodeId(), "containerID", s.GetContainerId())

	if s.GetNodeId() != c.nodeName {
		return nil
	}

	streamKey := events.LogKey{
		NodeID:      s.GetNodeId(),
		ContainerID: s.GetContainerId(),
		SessionID:   s.GetSessionId(),
	}

	c.logStreamsMu.Lock()
	cancel, exists := c.activeLogStreams[streamKey]
	c.logStreamsMu.Unlock()

	if exists {
		cancel()
		c.logger.Debug("cancelled log stream", "nodeID", s.GetNodeId(), "containerID", s.GetContainerId())
	}

	return nil
}

func (c *NodeController) onSchedule(ctx context.Context, obj *eventsv1.Event) error {
	ctx, span := c.tracer.Start(ctx, "controller.node.OnSchedule")
	defer span.End()

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

func (c *NodeController) onNodeConnect(ctx context.Context, e *eventsv1.Event) error {
	_, span := c.tracer.Start(ctx, "controller.node.OnNodeConnect")
	defer span.End()

	var n nodes.Node
	err := e.GetObject().UnmarshalTo(&n)
	if err != nil {
		return err
	}

	span.SetAttributes(
		attribute.String("client.id", e.GetClientId()),
		attribute.String("object.id", e.GetObjectId()),
		attribute.String("node.id", n.GetMeta().GetName()),
	)

	return nil
}

func (c *NodeController) onNodeCreate(ctx context.Context, _ *eventsv1.Event) error {
	return nil
}

func (c *NodeController) onNodeUpdate(ctx context.Context, obj *eventsv1.Event) error {
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

func (c *NodeController) onContainerDelete(ctx context.Context, e *eventsv1.Event) error {
	ctx, span := c.tracer.Start(ctx, "controller.node.OnContainerDelete")
	defer span.End()

	var ctr containersv1.Container
	err := e.GetObject().UnmarshalTo(&ctr)
	if err != nil {
		return err
	}

	containerID := ctr.GetMeta().GetName()
	c.logger.Info("controller received task", "event", e.GetType().String(), "name", ctr.GetMeta().GetName())

	err = c.runtime.Delete(ctx, &ctr)
	if err != nil {
		_ = c.clientset.ContainerV1().Status().Update(
			ctx,
			containerID,
			&containersv1.Status{
				Phase:  wrapperspb.String(consts.ERRDELETE),
				Status: wrapperspb.String(err.Error()),
			}, "phase", "status")
		return err
	}
	return nil
}

func (c *NodeController) onContainerUpdate(ctx context.Context, e *eventsv1.Event) error {
	ctx, span := c.tracer.Start(ctx, "controller.node.OnContainerUpdate")
	defer span.End()

	err := c.onContainerStop(ctx, e)
	if errors.IgnoreNotFound(err) != nil {
		return err
	}
	err = c.onContainerStart(ctx, e)
	if err != nil {
		return err
	}
	return nil
}

func (c *NodeController) onContainerKill(ctx context.Context, e *eventsv1.Event) error {
	ctx, span := c.tracer.Start(ctx, "controller.node.OnContainerKill")
	defer span.End()

	var ctr containersv1.Container
	err := e.GetObject().UnmarshalTo(&ctr)
	if err != nil {
		return err
	}

	containerID := ctr.GetMeta().GetName()
	c.logger.Info("controller received task", "event", e.GetType().String(), "name", containerID)

	_ = c.clientset.ContainerV1().Status().Update(ctx, containerID, &containersv1.Status{Phase: wrapperspb.String("stopping")}, "phase")
	err = c.runtime.Kill(ctx, &ctr)
	if errors.IgnoreNotFound(err) != nil {
		_ = c.clientset.ContainerV1().Status().Update(
			ctx,
			containerID,
			&containersv1.Status{
				Phase:  wrapperspb.String(consts.ERRKILL),
				Status: wrapperspb.String(err.Error()),
			}, "phase", "status")
		return err
	}

	err = c.onContainerDelete(ctx, e)
	if errors.IgnoreNotFound(err) != nil {
		return err
	}

	_ = c.clientset.ContainerV1().Status().Update(ctx, containerID, &containersv1.Status{Phase: wrapperspb.String(consts.PHASESTOPPED), Status: wrapperspb.String("")}, "phase", "status")

	return nil
}

func (c *NodeController) onContainerStart(ctx context.Context, e *eventsv1.Event) error {
	ctx, span := c.tracer.Start(ctx, "controller.node.OnContainerStart")
	defer span.End()

	var ctr containersv1.Container
	err := e.GetObject().UnmarshalTo(&ctr)
	if err != nil {
		return err
	}

	containerID := ctr.GetMeta().GetName()
	c.logger.Debug("controller received task", "event", e.GetType().String(), "name", containerID)

	_ = c.onContainerDelete(ctx, e)

	// Pull image
	_ = c.clientset.ContainerV1().Status().Update(ctx, containerID, &containersv1.Status{Phase: wrapperspb.String("pulling")}, "phase")
	err = c.runtime.Pull(ctx, &ctr)
	if err != nil {
		_ = c.clientset.ContainerV1().Status().Update(
			ctx,
			containerID,
			&containersv1.Status{
				Phase:  wrapperspb.String(consts.ERRIMAGEPULL),
				Status: wrapperspb.String(err.Error()),
			}, "phase", "status")
		return err
	}

	// Resolve volume reference in container spec
	mounts, err := c.resolvVolumeForMounts(ctx, ctr.GetConfig().GetMounts())
	if err != nil {
		return err
	}
	ctr.GetConfig().Mounts = mounts

	// Run container
	err = c.runtime.Run(ctx, &ctr)
	if err != nil {
		_ = c.clientset.ContainerV1().Status().Update(
			ctx,
			containerID,
			&containersv1.Status{
				Phase:  wrapperspb.String(consts.ERREXEC),
				Status: wrapperspb.String(err.Error()),
			}, "phase", "status")
		return err
	}

	_ = c.clientset.ContainerV1().Status().Update(ctx, containerID, &containersv1.Status{Phase: wrapperspb.String(consts.PHASERUNNING), Status: wrapperspb.String("")}, "phase", "status")

	return nil
}

func (c *NodeController) resolvVolumeForMounts(ctx context.Context, mounts []*containersv1.Mount) ([]*containersv1.Mount, error) {
	result := []*containersv1.Mount{}
	for _, mount := range mounts {
		v, err := c.clientset.VolumeV1().Get(ctx, mount.GetVolume())
		if err != nil {
			return nil, err
		}
		mount.Source = v.GetStatus().GetLocation().GetValue()
		result = append(result, mount)
	}
	return result, nil
}

func (c *NodeController) onContainerStop(ctx context.Context, e *eventsv1.Event) error {
	ctx, span := c.tracer.Start(ctx, "controller.node.OnContainerStop")
	defer span.End()

	var ctr containersv1.Container
	err := e.GetObject().UnmarshalTo(&ctr)
	if err != nil {
		return err
	}

	containerID := ctr.GetMeta().GetName()
	c.logger.Info("controller received task", "event", e.GetType().String(), "name", containerID)

	_ = c.clientset.ContainerV1().Status().Update(ctx, containerID, &containersv1.Status{Phase: wrapperspb.String("stopping")}, "phase")
	err = c.runtime.Stop(ctx, &ctr)
	if errors.IgnoreNotFound(err) != nil {
		_ = c.clientset.ContainerV1().Status().Update(
			ctx,
			containerID,
			&containersv1.Status{
				Phase:  wrapperspb.String(consts.ERRSTOP),
				Status: wrapperspb.String(err.Error()),
			}, "phase", "status")
		return err
	}

	err = c.onContainerDelete(ctx, e)
	if errors.IgnoreNotFound(err) != nil {
		return err
	}

	_ = c.clientset.ContainerV1().Status().Update(ctx, containerID, &containersv1.Status{Phase: wrapperspb.String(consts.PHASESTOPPED), Status: wrapperspb.String("")}, "phase", "status")

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
		tracer:                   otel.Tracer("controller"),
		logChan:                  make(chan *logsv1.LogEntry),
		activeLogStreams:         make(map[events.LogKey]context.CancelFunc),
	}
	for _, opt := range opts {
		opt(m)
	}

	return m, nil
}
