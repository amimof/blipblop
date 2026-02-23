package schedulercontroller

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/amimof/voiyd/internal/app"
	"github.com/amimof/voiyd/pkg/client"
	"github.com/amimof/voiyd/pkg/condition"
	"github.com/amimof/voiyd/pkg/errs"
	"github.com/amimof/voiyd/pkg/events"
	"github.com/amimof/voiyd/pkg/labels"
	"github.com/amimof/voiyd/pkg/logger"
	"github.com/amimof/voiyd/pkg/queue"
	"github.com/amimof/voiyd/pkg/scheduling"

	eventsv1 "github.com/amimof/voiyd/api/services/events/v1"
	leasesv1 "github.com/amimof/voiyd/api/services/leases/v1"
	nodesv1 "github.com/amimof/voiyd/api/services/nodes/v1"
	tasksv1 "github.com/amimof/voiyd/api/services/tasks/v1"
	typesv1 "github.com/amimof/voiyd/api/types/v1"
)

type NewOption func(c *Controller)

func WithLogger(l logger.Logger) NewOption {
	return func(c *Controller) {
		c.logger = l
	}
}

func WithExchange(e *events.Exchange) NewOption {
	return func(c *Controller) {
		c.exchange = e
	}
}

func WithNodeService(ns app.NodeSender) NewOption {
	return func(c *Controller) {
		c.nodeService = ns
	}
}

type Controller struct {
	clientset   *client.ClientSet
	scheduler   scheduling.Scheduler
	logger      logger.Logger
	exchange    *events.Exchange
	nodeService app.NodeSender
	workPool    *queue.WorkPool
	queue       *queue.TaskQueue
	publisher   condition.Publisher
}

func (c *Controller) Report(report *typesv1.ConditionReport) {
	status := report.GetStatus() == typesv1.ConditionStatus_CONDITION_STATUS_TRUE
	c.publisher.Report(report.GetResourceId(), status, report)
}

func (c *Controller) processScheduleTask(ctx context.Context, task *tasksv1.Task) error {
	c.logger.Info("enqueuing scheduling task", "task", task.GetMeta().GetName())
	t, err := c.clientset.TaskV1().Get(ctx, task.GetMeta().GetUid())
	if err != nil {
		return err
	}
	return c.queue.Enqueue(ctx, &queue.QueueItem{
		Task:       t,
		EnqueuedAt: time.Now(),
		RetryCount: 0,
		Handler:    c.scheduleTask,
	})
}

func (c *Controller) processTaskStop(ctx context.Context, task *tasksv1.Task) error {
	c.logger.Info("enqueuing stop task", "task", task.GetMeta().GetName())
	t, err := c.clientset.TaskV1().Get(ctx, task.GetMeta().GetUid())
	if err != nil {
		return err
	}
	return c.queue.Enqueue(ctx, &queue.QueueItem{
		Task:       t,
		EnqueuedAt: time.Now(),
		RetryCount: 0,
		Handler:    c.stopTask,
	})
}

func (c *Controller) processTaskKill(ctx context.Context, task *tasksv1.Task) error {
	c.logger.Info("enqueuing kill task", "task", task.GetMeta().GetName())
	t, err := c.clientset.TaskV1().Get(ctx, task.GetMeta().GetUid())
	if err != nil {
		return err
	}
	return c.queue.Enqueue(ctx, &queue.QueueItem{
		Task:       t,
		EnqueuedAt: time.Now(),
		RetryCount: 0,
		Handler:    c.killTask,
	})
}

func (c *Controller) processTaskUpdate(ctx context.Context, task *tasksv1.Task) error {
	lease, err := c.clientset.LeaseV1().Get(ctx, task.GetMeta().GetUid())
	if err != nil {
		if errs.IsNotFound(err) {
			return nil
		}
		return err
	}

	n, err := c.reschedule(ctx, task, lease.GetConfig().GetNodeId())
	if err != nil {
		if err := c.stop(ctx, task, lease.GetConfig().GetNodeId()); err != nil {
			fmt.Println("error sending stop to node", "error", err)
			return err
		}
		return err
	}

	c.logger.Debug("scheduled task on node", "node", n.GetMeta().GetName())

	return nil
}

func (c *Controller) reschedule(ctx context.Context, task *tasksv1.Task, nodeUID string) (*nodesv1.Node, error) {
	node, err := c.clientset.NodeV1().Get(ctx, nodeUID)
	if err != nil {
		return nil, err
	}

	selector := labels.NewCompositeSelectorFromMap(task.GetConfig().GetNodeSelector())
	if selector.Matches(node.GetMeta().GetLabels()) {
		return nil, nil
	}

	c.logger.Debug("rescheduling task due to selector mismatch", "task", task.GetMeta().GetName())

	// Release current lease
	err = c.clientset.LeaseV1().Revoke(ctx, task.GetMeta().GetUid(), node.GetMeta().GetUid())
	if err != nil {
		return nil, err
	}

	newNode, err := c.start(ctx, task)
	if err != nil {
		return nil, err
	}

	return newNode, err
}

func (c *Controller) stop(ctx context.Context, task *tasksv1.Task, nodeUID string) error {
	if !c.nodeService.IsNodeConnected(nodeUID) {
		c.logger.Warn("target node not connected, cannot stop", "task", task.GetMeta().GetName(), "node", nodeUID)
		return fmt.Errorf("target node %s not connected", nodeUID)
	}

	// Create kill event
	event := events.NewEvent(events.TaskStop, task)

	// Send to target node
	if err := c.nodeService.SendToNode(ctx, nodeUID, event); err != nil {
		c.logger.Error("failed to send stop event to node", "error", err, "task", task.GetMeta().GetName(), "node", nodeUID)
		return err
	}

	return nil
}

func (c *Controller) setTaskAsSchedulingFailed(ctx context.Context, task *tasksv1.Task, reason string) error {
	task.GetStatus().Phase = wrapperspb.String(string(condition.ReasonSchedulingFailed))
	task.GetStatus().Node = wrapperspb.String("")
	task.GetStatus().Reason = wrapperspb.String(reason)
	return c.clientset.TaskV1().Update(ctx, task.GetMeta().GetUid(), task)
}

func (c *Controller) setTaskAsScheduled(ctx context.Context, task *tasksv1.Task, nodeName string) error {
	task.GetStatus().Phase = wrapperspb.String(string(condition.ReasonScheduled))
	task.GetStatus().Node = wrapperspb.String(nodeName)
	return c.clientset.TaskV1().Update(ctx, task.GetMeta().GetUid(), task)
}

func (c *Controller) processLeaseExpired(ctx context.Context, lease *leasesv1.Lease) error {
	task, err := c.clientset.TaskV1().Get(ctx, lease.GetConfig().GetTaskId())
	if errs.IgnoreNotFound(err) != nil {
		return err
	}
	c.logger.Info("enqueuing lease expired task", "task", task.GetMeta().GetName())
	return c.queue.Enqueue(ctx, &queue.QueueItem{
		Task:       task,
		EnqueuedAt: time.Now(),
		RetryCount: 0,
		Handler:    c.scheduleTask,
	})
}

func (c *Controller) processNode(ctx context.Context, node *nodesv1.Node) error {
	tasks, err := c.clientset.TaskV1().List(ctx)
	if err != nil {
		c.logger.Error("error listing tasks on node join", "error", err)
		return err
	}
	for _, task := range tasks {
		taskID := task.GetMeta().GetUid()

		shouldSchedule, err := c.shouldScheduleTask(ctx, taskID)
		if err != nil {
			c.logger.Error("error checking if task should be scheduled",
				"error", err,
				"task", task.GetMeta().GetName())
			continue
		}

		if shouldSchedule {
			c.logger.Debug("scheduling task on node join",
				"task", task.GetMeta().GetName(),
				"node", node.GetMeta().GetName())

			if err := c.processScheduleTask(ctx, task); err != nil {
				c.logger.Warn("failed to schedule task on node join",
					"error", err,
					"task", task.GetMeta().GetName())
			}
		}
	}
	return nil
}

func (c *Controller) killTask(ctx context.Context, task *tasksv1.Task) error {
	reporter := condition.NewReportFor(task)

	lease, err := c.clientset.LeaseV1().Get(ctx, task.GetMeta().GetUid())
	if err != nil {
		return err
	}

	node, err := c.clientset.NodeV1().Get(ctx, lease.GetConfig().GetNodeId())
	if err != nil {
		return err
	}

	nodeUID := node.GetMeta().GetUid()

	if !c.nodeService.IsNodeConnected(nodeUID) {
		c.logger.Warn("target node not connected, cannot kill", "task", task.GetMeta().GetName(), "node", node.GetMeta().GetName())
		c.Report(reporter.Type(condition.TaskScheduled).False(condition.ReasonSchedulingFailed, "target node not connected"))
		return fmt.Errorf("target node %s not connected", node.GetMeta().GetName())
	}

	// Create kill event
	event := events.NewEvent(events.TaskKill, task)

	// Send to target node
	if err := c.nodeService.SendToNode(ctx, nodeUID, event); err != nil {
		c.logger.Error("failed to send kill event to node", "error", err, "task", task.GetMeta().GetName(), "node", node.GetMeta().GetName())
		c.Report(reporter.Type(condition.TaskReady).False(condition.ReasonStopFailed, err.Error()))
		return err
	}

	return c.setTaskAsScheduled(ctx, task, node.GetMeta().GetName())
}

func (c *Controller) stopTask(ctx context.Context, task *tasksv1.Task) error {
	reporter := condition.NewReportFor(task)

	lease, err := c.clientset.LeaseV1().Get(ctx, task.GetMeta().GetUid())
	if errs.IgnoreNotFound(err) != nil {
		return err
	}

	node, err := c.clientset.NodeV1().Get(ctx, lease.GetConfig().GetNodeId())
	if err != nil {
		return err
	}

	nodeUID := node.GetMeta().GetUid()

	if !c.nodeService.IsNodeConnected(nodeUID) {
		c.logger.Warn("target node not connected, cannot stop", "task", task.GetMeta().GetName(), "node", node.GetMeta().GetName())
		c.Report(reporter.Type(condition.TaskReady).False(condition.ReasonStopFailed, "target node not connected"))
		return fmt.Errorf("target node %s not connected", node.GetMeta().GetName())
	}

	// Create kill event
	event := events.NewEvent(events.TaskStop, task)

	// Send to target node
	if err := c.nodeService.SendToNode(ctx, nodeUID, event); err != nil {
		c.logger.Error("failed to send stop event to node", "error", err, "task", task.GetMeta().GetName(), "node", node.GetMeta().GetName())
		c.Report(reporter.Type(condition.TaskReady).False(condition.ReasonStopFailed, err.Error()))
		return err
	}

	// Update scheduling fields
	return c.setTaskAsScheduled(ctx, task, node.GetMeta().GetName())
}

func (c *Controller) scheduleTask(ctx context.Context, task *tasksv1.Task) error {
	_, err := c.start(ctx, task)
	if err != nil {
		return err
	}
	return nil
}

// scheduleTask attempts to schedule a task to a suitable node.
func (c *Controller) start(ctx context.Context, task *tasksv1.Task) (n *nodesv1.Node, err error) {
	taskID := task.GetMeta().GetUid()

	defer func() {
		if err != nil {
			if err := c.setTaskAsSchedulingFailed(ctx, task, n.GetMeta().GetName()); err != nil {
				c.logger.Warn("couldn't set mark task as scheduling failed", "error", err, "task", task.GetMeta().GetName())
			}
		}
	}()

	nodes, err := c.clientset.NodeV1().List(ctx)
	if err != nil {
		return nil, err
	}

	// Find a node fit for the task using a scheduler
	n, err = c.scheduler.Schedule(ctx, task, nodes)
	if err != nil {
		c.logger.Warn("error scheduling task", "task", taskID, "error", err)
		return nil, err
	}

	nodeUID := n.GetMeta().GetUid()

	// Check if target node is connected
	if !c.nodeService.IsNodeConnected(nodeUID) {
		c.logger.Warn("target node not connected, cannot schedule", "task", task.GetMeta().GetName(), "node", n.GetMeta().GetName())
		return nil, fmt.Errorf("target node %s not connected", n.GetMeta().GetName())
	}

	taskpb, err := anypb.New(task)
	if err != nil {
		return nil, err
	}

	nodepb, err := anypb.New(n)
	if err != nil {
		return nil, err
	}

	// Create targeted schedule request
	scheduleReq := &eventsv1.ScheduleRequest{
		Task: taskpb,
		Node: nodepb,
	}

	// Release current lease when starting a task
	err = c.clientset.LeaseV1().Revoke(ctx, task.GetMeta().GetUid(), nodeUID)
	if errs.IgnoreNotFound(err) != nil {
		return nil, err
	}

	// Create event
	event := events.NewEvent(events.Schedule, scheduleReq)
	if err := c.nodeService.SendToNode(ctx, nodeUID, event); err != nil {
		c.logger.Error("failed to send schedule event to node", "error", err, "task", task.GetMeta().GetName(), "node", n.GetMeta().GetName())
		return nil, err
	}

	// Update task status AFTER successful send
	c.logger.Info("scheduled task to node", "task", task.GetMeta().GetName(), "node", n.GetMeta().GetName())

	// Update scheduling fields
	err = c.setTaskAsScheduled(ctx, task, n.GetMeta().GetName())
	if err != nil {
		return nil, err
	}

	return n, nil
}

// shouldScheduleTask returns true if the task should be scheduled.
// A task should be scheduled if:
// - It has no lease (unscheduled)
// - Its lease has expired (past expiry + grace period)
func (c *Controller) shouldScheduleTask(ctx context.Context, taskID string) (bool, error) {
	lease, err := c.clientset.LeaseV1().Get(ctx, taskID)
	if err != nil {
		if errs.IsNotFound(err) {
			// No lease = unscheduled = should schedule
			return true, nil
		}
		// Other error
		return false, err
	}

	// Lease exists - check if expired (with grace period)
	const gracePeriod = 10 * time.Second
	expiryWithGrace := lease.GetConfig().GetExpiresAt().AsTime().Add(gracePeriod)
	leaseExpired := time.Now().After(expiryWithGrace)

	return leaseExpired, nil
}

func (c *Controller) Reconcile(ctx context.Context) error {
	return nil
}

func (c *Controller) Run(ctx context.Context) {
	// Subscribe to events
	ctx = metadata.AppendToOutgoingContext(ctx, "voiyd_controller_name", "scheduler")
	_, err := c.clientset.EventV1().Subscribe(ctx,
		events.TaskCreate,
		events.TaskUpdate,
		events.NodeConnect,
		events.NodeUpdate,
		events.NodePatch,
		events.LeaseExpired,
	)
	if err != nil {
		c.logger.Error("error subscribing to events", "error", err)
	}

	// Setup Handlers
	c.clientset.EventV1().On(events.TaskCreate, events.HandleErrors(c.logger, events.HandleTask(c.processScheduleTask)))
	c.clientset.EventV1().On(events.TaskStart, events.HandleErrors(c.logger, events.HandleTask(c.processScheduleTask)))
	c.clientset.EventV1().On(events.TaskUpdate, events.HandleErrors(c.logger, events.HandleTask(c.processTaskUpdate)))
	c.clientset.EventV1().On(events.TaskKill, events.HandleErrors(c.logger, events.HandleTask(c.processTaskKill)))
	c.clientset.EventV1().On(events.TaskStop, events.HandleErrors(c.logger, events.HandleTask(c.processTaskStop)))

	// NEW handlers
	c.clientset.EventV1().On(events.NodeConnect, events.HandleErrors(c.logger, events.HandleNode(c.processNode)))
	c.clientset.EventV1().On(events.NodeUpdate, events.HandleErrors(c.logger, events.HandleNode(c.processNode)))
	c.clientset.EventV1().On(events.NodePatch, events.HandleErrors(c.logger, events.HandleNode(c.processNode)))
	c.clientset.EventV1().On(events.NodeDelete, events.HandleErrors(c.logger, events.HandleNode(c.processNode)))

	// Setup lease handlers
	c.clientset.EventV1().On(events.LeaseExpired, events.HandleErrors(c.logger, events.HandleLease(c.processLeaseExpired)))

	go c.workPool.Start(ctx)

	publisher := condition.NewPublisher(c.clientset.TaskV1())
	go publisher.Run(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case e, ok := <-err:
			if !ok {
				err = nil
				continue
			}
			if e != nil {
				c.logger.Error("received error on channel", "error", e)
			}
		}
	}
}

func New(cs *client.ClientSet, opts ...NewOption) *Controller {
	c := &Controller{
		clientset: cs,
		logger:    logger.ConsoleLogger{},
		publisher: condition.NewPublisher(cs.TaskV1()),
		scheduler: scheduling.NewHorizontalScheduler(),
	}

	for _, opt := range opts {
		opt(c)
	}
	c.queue = queue.NewTaskQueue(c.logger)
	c.workPool = queue.NewPool(c.queue, queue.WithLogger(c.logger), queue.WithMaxRetries(5))

	return c
}
