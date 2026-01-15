// Package nodecontroller implemenets controller and provides logic for multiplexing node management
package nodecontroller

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	cevents "github.com/containerd/containerd/api/events"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/amimof/voiyd/pkg/client"
	"github.com/amimof/voiyd/pkg/consts"
	errs "github.com/amimof/voiyd/pkg/errors"
	"github.com/amimof/voiyd/pkg/events"
	"github.com/amimof/voiyd/pkg/logger"
	"github.com/amimof/voiyd/pkg/runtime"
	"github.com/amimof/voiyd/pkg/volume"

	eventsv1 "github.com/amimof/voiyd/api/services/events/v1"
	logsv1 "github.com/amimof/voiyd/api/services/logs/v1"
	nodesv1 "github.com/amimof/voiyd/api/services/nodes/v1"
	tasksv1 "github.com/amimof/voiyd/api/services/tasks/v1"
)

type Controller struct {
	runtime          runtime.Runtime
	logger           logger.Logger
	clientset        *client.ClientSet
	tracer           trace.Tracer
	logChan          chan *logsv1.LogEntry
	activeLogStreams map[events.LogKey]context.CancelFunc
	logStreamsMu     sync.Mutex
	node             *nodesv1.Node
	attacher         volume.Attacher
	exchange         *events.Exchange

	renewInterval time.Duration // 5s (half of TTL)
	leaseTTL      uint32        // 10s
}

type NewOption func(c *Controller)

func WithLeaseRenewalInterval(d time.Duration) NewOption {
	return func(c *Controller) {
		c.renewInterval = d
	}
}

func WithLeaseTTL(ttl uint32) NewOption {
	return func(c *Controller) {
		c.leaseTTL = ttl
	}
}

func WithVolumeAttacher(a volume.Attacher) NewOption {
	return func(c *Controller) {
		c.attacher = a
	}
}

func WithConfig(n *nodesv1.Node) NewOption {
	return func(c *Controller) {
		c.node = n
	}
}

func WithLogger(l logger.Logger) NewOption {
	return func(c *Controller) {
		c.logger = l
	}
}

func WithName(s string) NewOption {
	return func(c *Controller) {
		c.node.GetMeta().Name = s
	}
}

func WithExchange(e *events.Exchange) NewOption {
	return func(c *Controller) {
		c.exchange = e
	}
}

// Run implements controller
func (c *Controller) Run(ctx context.Context) {
	nodeName := c.node.GetMeta().GetName()
	// Subscribe to events
	ctx = metadata.AppendToOutgoingContext(ctx, "voiyd_controller_name", "node")
	evt, errCh := c.clientset.EventV1().Subscribe(ctx, events.ALL...)

	// Setup Node Handlers
	c.clientset.EventV1().On(events.NodeCreate, c.onNodeCreate)
	c.clientset.EventV1().On(events.NodeUpdate, c.handleErrors(c.onNodeUpdate))
	c.clientset.EventV1().On(events.NodeDelete, c.onNodeDelete)
	c.clientset.EventV1().On(events.NodeJoin, c.onNodeJoin)
	c.clientset.EventV1().On(events.NodeForget, c.onNodeForget)
	c.clientset.EventV1().On(events.NodeConnect, c.onNodeConnect)

	// Setup task handlers
	c.clientset.EventV1().On(events.TaskDelete, c.handleErrors(c.onTaskDelete))
	c.clientset.EventV1().On(events.TaskUpdate, c.handleErrors(c.onTaskUpdate))
	c.clientset.EventV1().On(events.TaskStop, c.handleErrors(c.onTaskStop))
	c.clientset.EventV1().On(events.TaskKill, c.handleErrors(c.onTaskKill))
	c.clientset.EventV1().On(events.TaskStart, c.handleErrors(c.onTaskStart))
	c.clientset.EventV1().On(events.Schedule, c.handleErrors(c.onSchedule))

	// Setup log handlers
	c.clientset.EventV1().On(events.TailLogsStart, c.handleErrors(c.onLogStart))
	c.clientset.EventV1().On(events.TailLogsStop, c.handleErrors(c.onLogStop))

	// Setup lease handlers
	c.clientset.EventV1().On(events.LeaseExpired, c.handleErrors(c.onLeaseExpired))

	go func() {
		for e := range evt {
			c.logger.Info("node controller received event", "event", e.GetType().String(), "clientID", nodeName, "objectID", e.GetObjectId())
		}
	}()

	// Handle runtime events
	runtimeChan := c.exchange.Subscribe(ctx, events.RuntimeTaskExit, events.RuntimeTaskStart)
	c.exchange.On(events.RuntimeTaskExit, c.handleErrors(c.onRuntimeTaskExit))
	c.exchange.On(events.RuntimeTaskStart, c.handleErrors(c.onRuntimeTaskStart))
	c.exchange.On(events.RuntimeTaskDelete, c.handleErrors(c.onRuntimeTaskDelete))

	go func() {
		for e := range runtimeChan {
			c.logger.Info("node controller received runtime event", "event", e.GetType().String(), "objectID", e.GetObjectId())
		}
	}()

	// Connect with retry logic
	connErr := make(chan error, 1)
	go func() {
		err := c.clientset.NodeV1().Connect(ctx, nodeName, evt, connErr)
		if err != nil {
			c.logger.Error("error connecting to server", "error", err)
		}
	}()

	// Reconcile
	go func() {
		if err := c.Reconcile(ctx); err != nil {
			c.logger.Warn("error reconciling", "error", err, "node", nodeName)
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
		nodeName, &nodesv1.Status{
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

	// Start lease loop
	go c.renewLeases(ctx)

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

// renewLeases continuously renews leases for all running tasks
func (c *Controller) renewLeases(ctx context.Context) {
	ticker := time.NewTicker(c.renewInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.renewAllLeases(ctx)
		}
	}
}

func (c *Controller) renewAllLeases(ctx context.Context) {
	nodeName := c.node.GetMeta().GetName()

	// Get all running tasks on this node from runtime
	tasks, err := c.runtime.List(ctx)
	if err != nil {
		c.logger.Error("failed to list runtime tasks", "error", err)
		return
	}

	for _, task := range tasks {
		taskName, err := c.runtime.Name(ctx, task.GetMeta().GetName())
		if err != nil {
			continue
		}

		// Renew lease
		renewed, err := c.clientset.LeaseV1().Renew(ctx, taskName, nodeName)
		if err != nil {
			c.logger.Error("error renewing lease", "error", err, "task", taskName, "node", nodeName)
			continue
		}

		if !renewed {
			c.logger.Warn("failed to renew lease", "task", taskName, "error", err)
		}

		c.logger.Debug("renewed lease, reconciling", "task", taskName)
	}
}

func (c *Controller) handleErrors(h events.HandlerFunc) events.HandlerFunc {
	return func(ctx context.Context, ev *eventsv1.Event) error {
		err := h(ctx, ev)
		if err != nil {
			c.logger.Error("handler returned error", "event", ev.GetType().String(), "error", err)
			return err
		}
		return err
	}
}

func (c *Controller) onLeaseExpired(ctx context.Context, obj *eventsv1.Event) error {
	var task tasksv1.Task
	err := obj.GetObject().UnmarshalTo(&task)
	if err != nil {
		return err
	}

	return c.startTask(ctx, &task)
}

func (c *Controller) onRuntimeTaskStart(ctx context.Context, obj *eventsv1.Event) error {
	var e cevents.TaskStart
	err := obj.GetObject().UnmarshalTo(&e)
	if err != nil {
		return err
	}

	tname, err := c.runtime.Name(ctx, e.GetContainerID())
	if err != nil {
		return err
	}

	nodeName := c.node.GetMeta().GetName()

	lease, err := c.clientset.LeaseV1().Get(ctx, tname)
	if err != nil {
		return err
	}

	// Only proceed if task is owned by us
	if lease.GetNodeId() == c.node.GetMeta().GetName() {
		c.logger.Info("received task start event from runtime", "task", e.GetContainerID(), "pid", e.GetPid())
		return c.clientset.TaskV1().Status().Update(
			ctx,
			tname,
			&tasksv1.Status{
				Phase:  wrapperspb.String(consts.PHASERUNNING),
				Reason: wrapperspb.String(""),
				Id:     wrapperspb.String(e.GetContainerID()),
				Pid:    wrapperspb.UInt32(e.GetPid()),
				Node:   wrapperspb.String(nodeName),
			}, "phase", "reason", "id", "pid", "node")
	}

	return nil
}

func (c *Controller) onRuntimeTaskExit(ctx context.Context, obj *eventsv1.Event) error {
	var e cevents.TaskExit
	err := obj.GetObject().UnmarshalTo(&e)
	if err != nil {
		return err
	}

	tname, err := c.runtime.Name(ctx, e.GetContainerID())
	if err != nil {
		return err
	}

	lease, err := c.clientset.LeaseV1().Get(ctx, tname)
	if err != nil {
		return err
	}

	// Only proceed if task is owned by us
	if lease.GetNodeId() == c.node.GetMeta().GetName() {
		c.logger.Info("received task exit event from runtime", "exitCode", e.GetExitStatus(), "pid", e.GetPid(), "exitedAt", e.GetExitedAt())
		phase := consts.PHASESTOPPED
		status := ""

		if e.GetExitStatus() > 0 {
			phase = consts.PHASEEXITED
			status = fmt.Sprintf("exit status %d", e.GetExitStatus())
		}

		return c.clientset.TaskV1().Status().Update(
			ctx,
			tname,
			&tasksv1.Status{
				Phase:  wrapperspb.String(phase),
				Reason: wrapperspb.String(status),
				Pid:    wrapperspb.UInt32(0),
				Id:     wrapperspb.String(""),
				Node:   wrapperspb.String(""),
			}, "phase", "reason", "pid", "id", "node")
	}

	return nil
}

func (c *Controller) onRuntimeTaskDelete(ctx context.Context, obj *eventsv1.Event) error {
	var e cevents.TaskDelete
	err := obj.GetObject().UnmarshalTo(&e)
	if err != nil {
		return err
	}

	tname, err := c.runtime.Name(ctx, e.GetContainerID())
	if err != nil {
		return err
	}

	lease, err := c.clientset.LeaseV1().Get(ctx, tname)
	if err != nil {
		return err
	}

	// Only proceed if task is owned by us
	if lease.GetNodeId() == c.node.GetMeta().GetName() {

		c.logger.Info("received task delete event from runtime", "task", e.GetContainerID(), "pid", e.GetPid())
		return c.clientset.TaskV1().Status().Update(
			ctx,
			tname,
			&tasksv1.Status{
				Phase:  wrapperspb.String(consts.PHASESTOPPED),
				Reason: wrapperspb.String(""),
				Id:     wrapperspb.String(""),
				Pid:    wrapperspb.UInt32(0),
				Node:   wrapperspb.String(""),
			}, "phase", "reason", "id", "pid", "node")
	}

	return nil
}

func (c *Controller) onLogStart(ctx context.Context, obj *eventsv1.Event) error {
	s := &logsv1.TailLogRequest{}
	err := obj.GetObject().UnmarshalTo(s)
	if err != nil {
		return err
	}
	c.logger.Debug("someone requested logs", "nodeID", s.GetNodeId(), "taskID", s.GetTaskId(), "sessionID", s.GetSessionId())

	if s.GetNodeId() != c.node.GetMeta().GetName() {
		return nil
	}

	streamKey := events.LogKey{
		NodeID:    s.GetNodeId(),
		TaskID:    s.GetTaskId(),
		SessionID: s.GetSessionId(),
	}

	c.logStreamsMu.Lock()
	if _, exists := c.activeLogStreams[streamKey]; exists {
		c.logStreamsMu.Unlock()
		c.logger.Debug("log stream already active", "nodeID", s.GetNodeId(), "taskID", s.GetTaskId(), "sessionID", s.GetSessionId())
		return nil
	}
	c.logStreamsMu.Unlock()

	taskIO, err := c.runtime.IO(ctx, s.GetTaskId())
	if err != nil {
		c.logger.Error("error getting task logs", "error", err)
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
		c.logger.Info("starting log scanner goroutine", "nodeID", s.GetNodeId(), "taskID", s.GetTaskId())

		defer func() {
			c.logStreamsMu.Lock()
			delete(c.activeLogStreams, streamKey)
			c.logStreamsMu.Unlock()
			cancel()
			_ = logStream.Close()
			if taskIO.Stdout != nil {
				_ = taskIO.Stdout.Close()
			}
		}()

		// Setup scanner. We use a channel to send each line through
		lines := make(chan string)
		scanner := bufio.NewScanner(taskIO.Stdout)

		// Goroutine that scans the log file and sends each line on the channel
		go func() {
			defer close(lines)

			for {

				// Exit early on cancel even when at EOF
				select {
				case <-streamCtx.Done():
					c.logger.Debug("exiting log streaming because context was cancelled", "nodeID", s.GetNodeId(), "taskID", s.GetTaskId())
					return
				default:
				}

				// Scan log file and send each line through the channel
				if scanner.Scan() {
					line := scanner.Text()
					select {
					case <-streamCtx.Done():
						c.logger.Debug("exiting log streaming because context was cancelled", "nodeID", s.GetNodeId(), "taskID", s.GetTaskId())
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
						"taskID", s.GetTaskId(),
					)
					return
				}

				// No errors, maybe EOF?
				select {
				case <-streamCtx.Done():
					c.logger.Debug("exiting log streaming because context was cancelled", "nodeID", s.GetNodeId(), "taskID", s.GetTaskId())
					return
				case <-time.After(300 * time.Millisecond): // Wait a bit before iterating again
				}

				// EOF reached, ovewrite the scanner to start reading again
				scanner = bufio.NewScanner(taskIO.Stdout)
			}
		}()

		// Count the number of lines read
		var seq uint64

		// Send log entry for each line that comes in from the line channel. After x amount of time anf if no lines are
		// received, exit out. This is a blocking operation.
		for {
			select {
			case <-streamCtx.Done():
				c.logger.Debug("log stream cancelled", "nodeID", s.GetNodeId(), "taskID", s.GetTaskId())
				return
			case <-time.After(5 * time.Minute):
				c.logger.Debug("scanner timeout - no data received", "nodeID", s.GetNodeId(), "taskID", s.GetTaskId())
				return
			case line, ok := <-lines:

				if !ok {
					c.logger.Debug("log stream completed", "nodeID", s.GetNodeId(), "taskID", s.GetTaskId())
					return
				}

				// Send the line as log entry to the server
				if err := logStream.Send(&logsv1.LogEntry{
					TaskId:    s.GetTaskId(),
					NodeId:    s.GetNodeId(),
					SessionId: s.GetSessionId(),
					Timestamp: timestamppb.Now(),
					Line:      line,
					Seq:       seq,
				}); err != nil {
					c.logger.Error(
						"error pushing log entry",
						"error", err,
						"taskID", s.GetTaskId(),
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

func (c *Controller) onLogStop(_ context.Context, obj *eventsv1.Event) error {
	s := &logsv1.TailLogRequest{}
	err := obj.GetObject().UnmarshalTo(s)
	if err != nil {
		return err
	}
	c.logger.Debug("someone requested stop logs", "nodeID", s.GetNodeId(), "taskID", s.GetTaskId())

	if s.GetNodeId() != c.node.GetMeta().GetName() {
		return nil
	}

	streamKey := events.LogKey{
		NodeID:    s.GetNodeId(),
		TaskID:    s.GetTaskId(),
		SessionID: s.GetSessionId(),
	}

	c.logStreamsMu.Lock()
	cancel, exists := c.activeLogStreams[streamKey]
	c.logStreamsMu.Unlock()

	if exists {
		cancel()
		c.logger.Debug("cancelled log stream", "nodeID", s.GetNodeId(), "taskID", s.GetTaskId())
	}

	return nil
}

func (c *Controller) onSchedule(ctx context.Context, obj *eventsv1.Event) error {
	ctx, span := c.tracer.Start(ctx, "controller.node.OnSchedule")
	defer span.End()

	// Unwrap schedule type from the event
	s := &eventsv1.ScheduleRequest{}
	err := obj.GetObject().UnmarshalTo(s)
	if err != nil {
		return err
	}

	// Unwrap the node from the event object
	n := &nodesv1.Node{}
	err = s.GetNode().UnmarshalTo(n)
	if err != nil {
		return err
	}

	// We only care if the event is for us
	if n.GetMeta().GetName() != c.node.GetMeta().GetName() {
		return nil
	}

	var task tasksv1.Task
	err = s.GetTask().UnmarshalTo(&task)
	if err != nil {
		return err
	}

	newObj := proto.Clone(obj).(*eventsv1.Event)
	newObj.Object, err = anypb.New(&task)
	if err != nil {
		return err
	}

	err = c.onTaskStart(ctx, newObj)
	if err != nil {
		return err
	}

	return nil
}

func (c *Controller) onNodeConnect(ctx context.Context, e *eventsv1.Event) error {
	_, span := c.tracer.Start(ctx, "controller.node.OnNodeConnect")
	defer span.End()

	var n nodesv1.Node
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

func (c *Controller) onNodeCreate(ctx context.Context, _ *eventsv1.Event) error {
	return nil
}

func (c *Controller) onNodeUpdate(ctx context.Context, obj *eventsv1.Event) error {
	return nil
}

func (c *Controller) onNodeDelete(ctx context.Context, obj *eventsv1.Event) error {
	if obj.GetMeta().GetName() != c.node.GetMeta().GetName() {
		return nil
	}
	err := c.clientset.NodeV1().Forget(ctx, obj.GetMeta().GetName())
	if err != nil {
		c.logger.Error("error unjoining node", "node", obj.GetMeta().GetName(), "error", err)
		return err
	}
	c.logger.Debug("successfully unjoined node", "node", obj.GetMeta().GetName())
	return nil
}

func (c *Controller) onNodeJoin(ctx context.Context, obj *eventsv1.Event) error {
	return nil
}

func (c *Controller) onNodeForget(ctx context.Context, obj *eventsv1.Event) error {
	return nil
}

func (c *Controller) onTaskDelete(ctx context.Context, e *eventsv1.Event) error {
	ctx, span := c.tracer.Start(ctx, "controller.node.OnTaskDelete")
	defer span.End()

	var task tasksv1.Task
	err := e.GetObject().UnmarshalTo(&task)
	if err != nil {
		return err
	}

	taskID := task.GetMeta().GetName()
	c.logger.Info("controller received task", "event", e.GetType().String(), "name", task.GetMeta().GetName())

	_ = c.clientset.TaskV1().Status().Update(ctx, taskID, &tasksv1.Status{Phase: wrapperspb.String(consts.PHASESTOPPING)}, "phase")
	err = c.runtime.Delete(ctx, &task)
	if err != nil {
		_ = c.clientset.TaskV1().Status().Update(
			ctx,
			taskID,
			&tasksv1.Status{
				Phase:  wrapperspb.String(consts.ERRDELETE),
				Reason: wrapperspb.String(err.Error()),
			}, "phase", "reason")
		return err
	}
	return nil
}

func (c *Controller) onTaskUpdate(ctx context.Context, e *eventsv1.Event) error {
	ctx, span := c.tracer.Start(ctx, "controller.node.OnTaskUpdate")
	defer span.End()

	err := c.onTaskStop(ctx, e)
	if errs.IgnoreNotFound(err) != nil {
		return err
	}
	err = c.onTaskStart(ctx, e)
	if err != nil {
		return err
	}
	return nil
}

func (c *Controller) onTaskKill(ctx context.Context, e *eventsv1.Event) error {
	ctx, span := c.tracer.Start(ctx, "controller.node.OnTaskKill")
	defer span.End()

	var task tasksv1.Task
	err := e.GetObject().UnmarshalTo(&task)
	if err != nil {
		return err
	}

	taskID := task.GetMeta().GetName()
	c.logger.Info("controller received task", "event", e.GetType().String(), "name", taskID)

	_ = c.clientset.TaskV1().Status().Update(ctx, taskID, &tasksv1.Status{Phase: wrapperspb.String(consts.PHASESTOPPING)}, "phase")
	err = c.runtime.Kill(ctx, &task)
	if errs.IgnoreNotFound(err) != nil {
		_ = c.clientset.TaskV1().Status().Update(
			ctx,
			taskID,
			&tasksv1.Status{
				Phase:  wrapperspb.String(consts.ERRKILL),
				Reason: wrapperspb.String(err.Error()),
			}, "phase", "status")
		return err
	}

	// Remove any previous tasks ignoring any errors
	err = c.runtime.Delete(ctx, &task)
	if err != nil {
		_ = c.clientset.TaskV1().Status().Update(
			ctx,
			taskID,
			&tasksv1.Status{
				Phase:  wrapperspb.String(consts.ERRDELETE),
				Reason: wrapperspb.String(err.Error()),
			}, "phase", "status")
		return err
	}

	return nil
}

func (c *Controller) startTask(ctx context.Context, task *tasksv1.Task) error {
	taskID := task.GetMeta().GetName()
	nodeID := c.node.GetMeta().GetName()

	expired, err := c.clientset.LeaseV1().Acquire(ctx, taskID, nodeID, c.leaseTTL)
	if err != nil {
		c.logger.Error("failed to acquire lease", "error", err, "task", taskID, "nodeID", nodeID)
		return err
	}

	if !expired {
		c.logger.Warn("lease held by another node", "task", taskID)
		return errors.New("lease held by another another")
	}

	c.logger.Info("acquired lease for task", "task", taskID, "node", nodeID)

	// Release if task can't be provisioned
	defer func() {
		if err != nil {
			err = c.clientset.LeaseV1().Release(ctx, taskID, nodeID)
			if err != nil {
				c.logger.Warn("unable to release lease", "task", taskID, "node", nodeID)
			}
		}
	}()

	// Run cleanup early while netns still exists.
	// This will allow the CNI plugin to remove networks without leaking.
	_ = c.runtime.Cleanup(ctx, taskID)

	// Remove any previous tasks ignoring any errors
	err = c.runtime.Delete(ctx, task)
	if err != nil {
		_ = c.clientset.TaskV1().Status().Update(
			ctx,
			taskID,
			&tasksv1.Status{
				Phase:  wrapperspb.String(consts.ERRDELETE),
				Reason: wrapperspb.String(err.Error()),
			}, "phase", "status")
		return err
	}

	// Prepare volumes/mounts
	if err := c.attacher.PrepareMounts(ctx, c.node, task); err != nil {
		_ = c.clientset.TaskV1().Status().Update(
			ctx,
			taskID,
			&tasksv1.Status{
				Phase:  wrapperspb.String(consts.ERREXEC),
				Reason: wrapperspb.String(err.Error()),
			}, "phase", "status")
		return err
	}

	// Pull image
	_ = c.clientset.TaskV1().Status().Update(ctx, taskID, &tasksv1.Status{Phase: wrapperspb.String(consts.PHASEPULLING)}, "phase")
	err = c.runtime.Pull(ctx, task)
	if err != nil {
		_ = c.clientset.TaskV1().Status().Update(
			ctx,
			taskID,
			&tasksv1.Status{
				Phase:  wrapperspb.String(consts.ERRIMAGEPULL),
				Reason: wrapperspb.String(err.Error()),
			}, "phase", "status")
		return err
	}

	// Run task
	_ = c.clientset.TaskV1().Status().Update(ctx, taskID, &tasksv1.Status{Phase: wrapperspb.String(consts.PHASESTARTING)}, "phase")
	err = c.runtime.Run(ctx, task)
	if err != nil {
		_ = c.clientset.TaskV1().Status().Update(
			ctx,
			taskID,
			&tasksv1.Status{
				Phase:  wrapperspb.String(consts.ERREXEC),
				Reason: wrapperspb.String(err.Error()),
			}, "phase", "status")
		return err
	}

	return nil
}

func (c *Controller) onTaskStart(ctx context.Context, e *eventsv1.Event) error {
	ctx, span := c.tracer.Start(ctx, "controller.node.OnTaskStart")
	defer span.End()

	var task tasksv1.Task
	err := e.GetObject().UnmarshalTo(&task)
	if err != nil {
		return err
	}

	c.logger.Debug("controller received task", "event", e.GetType().String(), "name", task.GetMeta().GetName())

	return c.startTask(ctx, &task)
}

func (c *Controller) onTaskStop(ctx context.Context, e *eventsv1.Event) error {
	ctx, span := c.tracer.Start(ctx, "controller.node.OnTaskStop")
	defer span.End()

	var task tasksv1.Task
	err := e.GetObject().UnmarshalTo(&task)
	if err != nil {
		return err
	}

	taskID := task.GetMeta().GetName()
	nodeID := c.node.GetMeta().GetName()

	// Release lease
	defer func() {
		err = c.clientset.LeaseV1().Release(ctx, taskID, nodeID)
		if err != nil {
			c.logger.Warn("unable to release lease", "error", err, "task", taskID, "nodeID", nodeID)
		}
	}()

	c.logger.Info("controller received task", "event", e.GetType().String(), "name", taskID)

	// Run cleanup early while netns still exists.
	// This will allow the CNI plugin to remove networks without leaking.
	err = c.runtime.Cleanup(ctx, taskID)
	if err != nil {
		return err
	}

	// Let everyone know that task is stoping
	_ = c.clientset.TaskV1().Status().Update(ctx, taskID, &tasksv1.Status{Phase: wrapperspb.String(consts.PHASESTOPPING)}, "phase")

	// Stop the task
	err = c.runtime.Stop(ctx, &task)
	if errs.IgnoreNotFound(err) != nil {
		_ = c.clientset.TaskV1().Status().Update(
			ctx,
			taskID,
			&tasksv1.Status{
				Phase:  wrapperspb.String(consts.ERRSTOP),
				Reason: wrapperspb.String(err.Error()),
			}, "phase", "status")
		return err
	}

	// Remove any previous tasks ignoring any errors
	err = c.runtime.Delete(ctx, &task)
	if err != nil {
		_ = c.clientset.TaskV1().Status().Update(
			ctx,
			taskID,
			&tasksv1.Status{
				Phase:  wrapperspb.String(consts.ERRDELETE),
				Reason: wrapperspb.String(err.Error()),
			}, "phase", "status")
		return err
	}

	// Detach volumes
	return c.attacher.Detach(ctx, c.node, &task)
}

// Reconcile ensures that desired tasks matches with tasks
// in the runtime environment. It removes any tasks that are not
// desired (missing from the server) and adds those missing from runtime.
// It is preferrably run early during startup of the controller.
func (c *Controller) Reconcile(ctx context.Context) error {
	nodeID := c.node.GetMeta().GetName()

	// Get running tasks from runtime
	runtimeTasks, err := c.runtime.List(ctx)
	if err != nil {
		return err
	}

	// Verify that the containers in the runtime are supposed to run. If a lease
	// for a running container cannot be acquired, stop it. Otherwise let it run.
	for _, task := range runtimeTasks {
		taskID := task.GetMeta().GetName()

		// Only acquire if task is supposed to be running
		if task.GetStatus().GetPhase().GetValue() != consts.PHASERUNNING {
			c.logger.Debug("skip lease acquisition because task is not running", "task", taskID)
			continue
		}

		// Try to acquire lease for this task
		acquired, err := c.clientset.LeaseV1().Acquire(ctx, taskID, nodeID, c.leaseTTL)
		if err != nil {
			c.logger.Error("error acquiring lease during reconcile", "task", taskID, "error", err)
			continue
		}

		// Successfully acquired lease - we can keep running this task
		if acquired {
			c.logger.Info("acquierd lease for task", "task", taskID, "node", nodeID, "ttl", c.leaseTTL)

			// Update task status to reflect actual state
			if err = c.clientset.TaskV1().Status().Update(ctx, taskID, &tasksv1.Status{
				Node: wrapperspb.String(nodeID),
			}, "node"); err != nil {
				c.logger.Warn("unable to update status", "error", err, "task", taskID, "node", nodeID)
			}

			continue
		}

		// Lease held by another node, stop running tasks in runtime
		c.logger.Warn("lease held by another node", "task", taskID, "node", nodeID)

		// Run cleanup early while netns still exists.
		// This will allow the CNI plugin to remove networks without leaking.
		err = c.runtime.Cleanup(ctx, taskID)
		if err != nil {
			return err
		}

		// Stop the task
		err = c.runtime.Stop(ctx, task)
		if errs.IgnoreNotFound(err) != nil {
			return err
		}

		// Remove any previous tasks ignoring any errors
		err = c.runtime.Delete(ctx, task)
		if err != nil {
			return err
		}

		// Detach volumes
		return c.attacher.Detach(ctx, c.node, task)

	}

	return nil
}

func New(c *client.ClientSet, n *nodesv1.Node, rt runtime.Runtime, opts ...NewOption) (*Controller, error) {
	m := &Controller{
		clientset:        c,
		runtime:          rt,
		logger:           logger.ConsoleLogger{},
		tracer:           otel.Tracer("controller"),
		logChan:          make(chan *logsv1.LogEntry),
		activeLogStreams: make(map[events.LogKey]context.CancelFunc),
		node:             n,
		attacher:         volume.NewDefaultAttacher(c.VolumeV1()),
		renewInterval:    time.Second * 10,
	}
	for _, opt := range opts {
		opt(m)
	}

	return m, nil
}
