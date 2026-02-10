package schedulercontroller

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/amimof/voiyd/pkg/client"
	"github.com/amimof/voiyd/pkg/condition"
	errs "github.com/amimof/voiyd/pkg/errors"
	"github.com/amimof/voiyd/pkg/events"
	"github.com/amimof/voiyd/pkg/labels"
	"github.com/amimof/voiyd/pkg/logger"
	"github.com/amimof/voiyd/pkg/queue"
	"github.com/amimof/voiyd/pkg/scheduling"
	"github.com/amimof/voiyd/services/node"

	eventsv1 "github.com/amimof/voiyd/api/services/events/v1"
	leasesv1 "github.com/amimof/voiyd/api/services/leases/v1"
	nodesv1 "github.com/amimof/voiyd/api/services/nodes/v1"
	tasksv1 "github.com/amimof/voiyd/api/services/tasks/v1"
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

func WithNodeService(ns *node.NodeService) NewOption {
	return func(c *Controller) {
		c.nodeService = ns
	}
}

type Controller struct {
	clientset   *client.ClientSet
	scheduler   scheduling.Scheduler
	logger      logger.Logger
	exchange    *events.Exchange
	nodeService *node.NodeService
	workPool    *queue.WorkPool
	queue       *queue.TaskQueue
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

func (c *Controller) processStopTask(ctx context.Context, task *tasksv1.Task) error {
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

func (c *Controller) processKillTask(ctx context.Context, task *tasksv1.Task) error {
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

func (c *Controller) processTaskLabelChange(ctx context.Context, task *tasksv1.Task) error {
	c.logger.Info("enqueuing label change task", "task", task.GetMeta().GetName())
	t, err := c.clientset.TaskV1().Get(ctx, task.GetMeta().GetUid())
	if err != nil {
		return err
	}

	taskID := t.GetMeta().GetUid()
	locked := true

	// Get current lease holder. If lease doesn exist == no lock and task can be scheduled
	lease, err := c.clientset.LeaseV1().Get(ctx, taskID)
	if err != nil {
		if errs.IsNotFound(err) {
			locked = false
		} else {
			return err
		}
	}

	// If task labels change such that it cannot continue to run on the current node
	// then release the lease and emit schedule event so it can be scheduled elsewhere
	if locked {
		node, err := c.clientset.NodeV1().Get(ctx, lease.GetConfig().GetNodeId())
		if err != nil {
			return err
		}
		selector := labels.NewCompositeSelectorFromMap(task.GetConfig().GetNodeSelector())
		if !selector.Matches(node.GetMeta().GetLabels()) {
			if err := c.releaseLeaseIfExists(ctx, taskID); err != nil {
				c.logger.Warn("error releasing lease for task", "error", err, "task", taskID)
			}
		}
	}

	return c.queue.Enqueue(ctx, &queue.QueueItem{
		Task:       t,
		EnqueuedAt: time.Now(),
		RetryCount: 0,
		Handler:    c.scheduleTask,
	})
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

func (c *Controller) releaseLeaseIfExists(ctx context.Context, taskID string) error {
	// Try to get the lease
	lease, err := c.clientset.LeaseV1().Get(ctx, taskID)
	if err != nil {
		if errs.IsNotFound(err) {
			c.logger.Debug("no lease to release", "task", taskID)
		} else {
			c.logger.Warn("error getting lease for release, skipping", "error", err, "task", taskID)
		}
		return nil
	}
	// Lease exists, attempt to release it
	nodeID := lease.GetConfig().GetNodeId()
	err = c.clientset.LeaseV1().Release(ctx, taskID, nodeID)
	if err != nil {
		if errs.IsNotFound(err) {
			c.logger.Debug("lease already released", "task", taskID, "node", nodeID)
		} else {
			c.logger.Warn("error releasing lease, continuing anyway", "error", err, "task", taskID, "node", nodeID)
		}
		return nil
	}
	c.logger.Info("released lease", "task", taskID, "node", nodeID)
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
		_ = c.clientset.TaskV1().Condition(ctx, reporter.Type(condition.TaskScheduled).False(condition.ReasonSchedulingFailed, "target node not connected"))
		return fmt.Errorf("target node %s not connected", node.GetMeta().GetName())
	}

	// Create kill event
	event := events.NewEvent(events.TaskKill, task)

	// Send to target node
	if err := c.nodeService.SendToNode(nodeUID, event); err != nil {
		c.logger.Error("failed to send kill event to node", "error", err, "task", task.GetMeta().GetName(), "node", node.GetMeta().GetName())
		_ = c.clientset.TaskV1().Condition(ctx, reporter.Type(condition.TaskReady).False(condition.ReasonStopFailed, err.Error()))
		return err
	}

	return nil
}

func (c *Controller) stopTask(ctx context.Context, task *tasksv1.Task) error {
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
		c.logger.Warn("target node not connected, cannot stop", "task", task.GetMeta().GetName(), "node", node.GetMeta().GetName())
		_ = c.clientset.TaskV1().Condition(ctx, reporter.Type(condition.TaskReady).False(condition.ReasonStopFailed, "target node not connected"))
		return fmt.Errorf("target node %s not connected", node.GetMeta().GetName())
	}

	// Create kill event
	event := events.NewEvent(events.TaskStop, task)

	// Send to target node
	if err := c.nodeService.SendToNode(nodeUID, event); err != nil {
		c.logger.Error("failed to send stop event to node", "error", err, "task", task.GetMeta().GetName(), "node", node.GetMeta().GetName())
		_ = c.clientset.TaskV1().Condition(ctx, reporter.Type(condition.TaskReady).False(condition.ReasonStopFailed, err.Error()))
		return err
	}

	return nil
}

// scheduleTask attempts to schedule a task to a suitable node.
//
// Scheduling flow:
// 1. Check if nodes match task's nodeSelector - if not, release lease and exit
// 2. Call horizontal scheduler to find best Ready node - if none, release lease and exit
// 3. Verify target node is connected - if not, release lease and exit
// 4. Send task to target node via targeted channel
//
// Rescheduling triggers:
// - NodeConnect event: onNodeJoin() reschedules tasks without leases
// - NodeUpdate/NodePatch event: onNodeLabelsChange() reschedules if labels now match
// - TaskUpdate event: processTaskLabelChange() reschedules if nodeSelector changes
// - LeaseExpired event: onLeaseExpired() reschedules tasks (safety net)
// - Queue retry: WorkPool retries failed tasks with exponential backoff
//
// When scheduling fails, the lease is released to signal the task is unscheduled.
// This allows event handlers to detect and reschedule the task when conditions improve.
func (c *Controller) scheduleTask(ctx context.Context, task *tasksv1.Task) error {
	taskID := task.GetMeta().GetUid()
	reporter := condition.NewReportFor(task)

	// Check if task has any nodes available for scheduling based on the tasks' selector
	match, err := c.hasMatchingNodes(ctx, task)
	if err != nil {
		c.logger.Debug("error matching selector with nodes", "error", err, "task", taskID, "selector", task.GetConfig().GetNodeSelector())
		_ = c.clientset.TaskV1().Condition(ctx, reporter.Type(condition.TaskScheduled).False(condition.ReasonSchedulingFailed, err.Error()))
		return err
	}

	// If no nodes matches selector, set status and exit
	if !match {
		c.logger.Debug("no nodes match task's nodeSelector", "task", taskID, "selector", task.GetConfig().GetNodeSelector())

		// Release any existing lease
		if err := c.releaseLeaseIfExists(ctx, taskID); err != nil {
			c.logger.Warn("error releasing lease for task", "error", err, "task", taskID)
			// Continue to set condition even if release fails
		}

		_ = c.clientset.TaskV1().Condition(ctx, reporter.Type(condition.TaskScheduled).False(condition.ReasonSchedulingFailed, "no nodes match node selector"))
		return scheduling.ErrSchedulingNoMatchingNode
	}

	// Find a node fit for the task using a scheduler
	n, err := c.scheduler.Schedule(ctx, task)
	if err != nil {
		c.logger.Debug("scheduling failed", "task", taskID, "error", err)

		// Release any existing lease so task can be rescheduled when nodes become available
		if releaseErr := c.releaseLeaseIfExists(ctx, taskID); releaseErr != nil {
			c.logger.Warn("error releasing lease after scheduling failure", "error", releaseErr, "task", taskID)
		}

		_ = c.clientset.TaskV1().Condition(ctx, reporter.Type(condition.TaskScheduled).False(condition.ReasonSchedulingFailed, err.Error()))
		return err
	}

	nodeUID := n.GetMeta().GetUid()
	md := map[string]string{
		"node": n.GetMeta().GetName(),
	}

	// Check if target node is connected
	if !c.nodeService.IsNodeConnected(nodeUID) {
		c.logger.Warn("target node not connected, cannot schedule", "task", task.GetMeta().GetName(), "node", n.GetMeta().GetName())

		// Release lease so task can be rescheduled on a different node
		if err := c.releaseLeaseIfExists(ctx, taskID); err != nil {
			c.logger.Warn("error releasing lease for disconnected node", "error", err, "task", taskID)
		}

		_ = c.clientset.TaskV1().Condition(ctx, reporter.Type(condition.TaskScheduled).False(condition.ReasonSchedulingFailed, "target node not connected"))
		return fmt.Errorf("target node %s not connected", n.GetMeta().GetName())
	}

	taskpb, err := anypb.New(task)
	if err != nil {
		return err
	}

	nodepb, err := anypb.New(n)
	if err != nil {
		return err
	}

	// Create targeted schedule request
	scheduleReq := &eventsv1.ScheduleRequest{
		Task: taskpb,
		Node: nodepb,
	}

	// Create event
	event := events.NewEvent(events.Schedule, scheduleReq)
	// Send ONLY to target node
	if err := c.nodeService.SendToNode(nodeUID, event); err != nil {
		c.logger.Error("failed to send schedule event to node", "error", err, "task", task.GetMeta().GetName(), "node", n.GetMeta().GetName())
		_ = c.clientset.TaskV1().Condition(ctx, reporter.Type(condition.TaskScheduled).False(condition.ReasonSchedulingFailed, err.Error()))
		return err
	}

	// Update task status AFTER successful send
	_ = c.clientset.TaskV1().Condition(ctx, reporter.Type(condition.TaskScheduled).WithMetadata(md).True(condition.ReasonScheduled, ""))
	c.logger.Info("scheduled task to node", "task", task.GetMeta().GetName(), "node", n.GetMeta().GetName())
	return nil
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

// Checks if there are nodes that matches the task's nodeSelector.
// Returns true if at least one node has matching labels.
// Returns false if no nodes has matching labels.
func (c *Controller) hasMatchingNodes(ctx context.Context, task *tasksv1.Task) (bool, error) {
	nodes, err := c.clientset.NodeV1().List(ctx)
	if err != nil {
		return false, err
	}

	// Check if any node matches the task's nodeSelector
	selector := labels.NewCompositeSelectorFromMap(task.GetConfig().GetNodeSelector())
	for _, node := range nodes {
		if selector.Matches(node.GetMeta().GetLabels()) {
			return true, nil
		}
	}

	return false, nil
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
	c.clientset.EventV1().On(events.TaskUpdate, events.HandleErrors(c.logger, events.HandleTask(c.processTaskLabelChange)))
	c.clientset.EventV1().On(events.TaskKill, events.HandleErrors(c.logger, events.HandleTask(c.processKillTask)))
	c.clientset.EventV1().On(events.TaskStop, events.HandleErrors(c.logger, events.HandleTask(c.processStopTask)))

	// NEW handlers
	c.clientset.EventV1().On(events.NodeConnect, events.HandleErrors(c.logger, events.HandleNode(c.processNode)))
	c.clientset.EventV1().On(events.NodeUpdate, events.HandleErrors(c.logger, events.HandleNode(c.processNode)))
	c.clientset.EventV1().On(events.NodePatch, events.HandleErrors(c.logger, events.HandleNode(c.processNode)))
	c.clientset.EventV1().On(events.NodeDelete, events.HandleErrors(c.logger, events.HandleNode(c.processNode)))

	// Setup lease handlers
	c.clientset.EventV1().On(events.LeaseExpired, events.HandleErrors(c.logger, events.HandleLease(c.processLeaseExpired)))

	go c.workPool.Start(ctx)

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

func New(cs *client.ClientSet, scheduler scheduling.Scheduler, opts ...NewOption) *Controller {
	c := &Controller{
		clientset: cs,
		scheduler: scheduler,
		logger:    logger.ConsoleLogger{},
	}

	for _, opt := range opts {
		opt(c)
	}
	c.queue = queue.NewTaskQueue(c.logger)
	c.workPool = queue.NewPool(c.queue, queue.WithLogger(c.logger))

	return c
}
