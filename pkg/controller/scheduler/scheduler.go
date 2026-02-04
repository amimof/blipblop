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
}

func (c *Controller) onLeaseExpired(ctx context.Context, lease *leasesv1.Lease) error {
	task, err := c.clientset.TaskV1().Get(ctx, lease.GetConfig().GetTaskId())
	if errs.IgnoreNotFound(err) != nil {
		return err
	}
	return c.scheduleTask(ctx, task)
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
		c.logger.Debug("no nodes matches task's nodeSelector", "task", taskID, "selector", task.GetConfig().GetNodeSelector())
		lease, err := c.clientset.LeaseV1().Get(ctx, taskID)
		if errs.IgnoreNotFound(err) != nil {
			c.logger.Error("error getting lease for task", "error", err, "taskID", taskID)
			return err
		}
		err = c.clientset.LeaseV1().Release(ctx, taskID, lease.GetConfig().GetNodeId())
		if errs.IgnoreNotFound(err) != nil {
			c.logger.Error("error releasing lease for task", "error", err, "task", taskID)
			_ = c.clientset.TaskV1().Condition(ctx, reporter.Type(condition.TaskScheduled).False(condition.ReasonSchedulingFailed, err.Error()))
			return err
		}
		return c.clientset.TaskV1().Condition(ctx, reporter.Type(condition.TaskScheduled).False(condition.ReasonSchedulingFailed, "no nodes matches node selector"))
	}

	// Find a node fit for the task using a scheduler
	n, err := c.scheduler.Schedule(ctx, task)
	if err != nil {
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

func (c *Controller) onNodeDelete(ctx context.Context, node *nodesv1.Node) error {
	tasks, err := c.clientset.TaskV1().List(ctx)
	if err != nil {
		return nil
	}

	for _, task := range tasks {

		reporter := condition.NewReportFor(task)
		l, err := c.clientset.LeaseV1().Get(ctx, task.GetMeta().GetUid())

		if errs.IgnoreNotFound(err) != nil {
			c.logger.Error("error getting lease", "error", err, "task", task.GetMeta().GetName(), "node", node.GetMeta().GetName())
			continue
		}

		// If lease is held by the deleted node, release
		if l.GetConfig().GetNodeId() == node.GetMeta().GetUid() {

			err = c.clientset.LeaseV1().Release(ctx, task.GetMeta().GetUid(), l.GetConfig().GetNodeId())
			if errs.IgnoreNotFound(err) != nil {
				c.logger.Error("error releasing lease for task", "error", err, "task", task.GetMeta().GetName())
				_ = c.clientset.TaskV1().Condition(ctx, reporter.Type(condition.TaskScheduled).False(condition.ReasonSchedulingFailed, err.Error()))
				return err
			}
			return c.scheduleTask(ctx, task)
		}

	}

	return nil
}

func (c *Controller) onNodeJoin(ctx context.Context, node *nodesv1.Node) error {
	tasks, err := c.clientset.TaskV1().List(ctx)
	if err != nil {
		return nil
	}

	for _, task := range tasks {
		l, err := c.clientset.LeaseV1().Get(ctx, task.GetMeta().GetUid())
		if errs.IgnoreNotFound(err) != nil {
			c.logger.Error("error getting lease", "error", err, "task", task.GetMeta().GetName(), "node", node.GetMeta().GetName())
			continue
		}

		// If lease expired means that task should be rescheduled
		if !time.Now().Before(l.GetConfig().GetExpiresAt().AsTime().Add(time.Second * 10)) {
			c.logger.Debug("emitting task start", "task", task.GetMeta().GetName())

			return c.scheduleTask(ctx, task)
		}
	}

	return nil
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

func (c *Controller) onTaskLabelsChange(ctx context.Context, task *tasksv1.Task) error {
	// Get current lease holder
	lease, err := c.clientset.LeaseV1().Get(ctx, task.GetMeta().GetUid())
	if err != nil {
		if errs.IsNotFound(err) {
			return nil
		}
		c.logger.Error("error getting lease", "error", err, "task", task.GetMeta().GetName())
	}

	node, err := c.clientset.NodeV1().Get(ctx, lease.GetConfig().GetNodeId())
	if err != nil {
		return err
	}

	// If task labels change such that it cannot continue to run on the current node
	// then release the lease and emit schedule event so it can be scheduled elsewhere
	selector := labels.NewCompositeSelectorFromMap(task.GetConfig().GetNodeSelector())
	if !selector.Matches(node.GetMeta().GetLabels()) {
		reporter := condition.NewForResource(task)
		err = c.clientset.LeaseV1().Release(ctx, task.GetMeta().GetUid(), node.GetMeta().GetUid())
		if errs.IgnoreNotFound(err) != nil {
			c.logger.Error("error releasing lease for task", "error", err, "task", task.GetMeta().GetName())
			_ = c.clientset.TaskV1().Condition(ctx, reporter.Type(condition.TaskScheduled).False(condition.ReasonSchedulingFailed, err.Error()))
			return err
		}
	}

	return c.scheduleTask(ctx, task)
}

func (c *Controller) onNodeLabelsChange(ctx context.Context, node *nodesv1.Node) error {
	tasks, err := c.clientset.TaskV1().List(ctx)
	if err != nil {
		return err
	}

	for _, task := range tasks {

		taskID := task.GetMeta().GetUid()

		// Skip tasks without node selector
		if task.GetConfig().GetNodeSelector() == nil || len(task.GetConfig().GetNodeSelector()) == 0 {
			c.logger.Debug("skipping because task has no node selector", "task", taskID)
			continue
		}

		// Get current lease
		lease, err := c.clientset.LeaseV1().Get(ctx, taskID)
		if err != nil {
			if errs.IsNotFound(err) {
				return c.scheduleTask(ctx, task)
			}
			c.logger.Error("error getting lease", "error", err, "task", taskID)
			continue
		}

		currentNodeID := lease.GetConfig().GetNodeId()

		// Skip if task is running on another node
		if currentNodeID != node.GetMeta().GetUid() {
			continue
		}

		if err := c.scheduleTask(ctx, task); err != nil {
			c.logger.Error("error forwarding task start event", "error", err, "task", taskID)
			return err
		}
	}
	return nil
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
	c.clientset.EventV1().On(events.TaskCreate, events.HandleErrors(c.logger, events.HandleTask(c.scheduleTask)))
	c.clientset.EventV1().On(events.TaskStart, events.HandleErrors(c.logger, events.HandleTask(c.scheduleTask)))
	c.clientset.EventV1().On(events.TaskUpdate, events.HandleErrors(c.logger, events.HandleTask(c.onTaskLabelsChange)))
	c.clientset.EventV1().On(events.TaskKill, events.HandleErrors(c.logger, events.HandleTask(c.killTask)))
	c.clientset.EventV1().On(events.TaskStop, events.HandleErrors(c.logger, events.HandleTask(c.stopTask)))

	// NEW handlers
	c.clientset.EventV1().On(events.NodeConnect, events.HandleErrors(c.logger, events.HandleNode(c.onNodeJoin)))
	c.clientset.EventV1().On(events.NodeUpdate, events.HandleErrors(c.logger, events.HandleNode(c.onNodeLabelsChange)))
	c.clientset.EventV1().On(events.NodePatch, events.HandleErrors(c.logger, events.HandleNode(c.onNodeLabelsChange)))
	c.clientset.EventV1().On(events.NodeDelete, events.HandleErrors(c.logger, events.HandleNode(c.onNodeDelete)))

	// Setup lease handlers
	c.clientset.EventV1().On(events.LeaseExpired, events.HandleErrors(c.logger, events.HandleLease(c.onLeaseExpired)))

	// Handle errors
	for e := range err {
		c.logger.Error("received error on channel", "error", e)
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

	return c
}
