package schedulercontroller

import (
	"context"
	"time"

	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/amimof/voiyd/pkg/client"
	"github.com/amimof/voiyd/pkg/consts"
	errs "github.com/amimof/voiyd/pkg/errors"
	"github.com/amimof/voiyd/pkg/events"
	"github.com/amimof/voiyd/pkg/labels"
	"github.com/amimof/voiyd/pkg/logger"
	"github.com/amimof/voiyd/pkg/scheduling"

	eventsv1 "github.com/amimof/voiyd/api/services/events/v1"
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

type Controller struct {
	clientset *client.ClientSet
	scheduler scheduling.Scheduler
	logger    logger.Logger
	exchange  *events.Exchange
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

func (c *Controller) onTaskUpdate(ctx context.Context, e *eventsv1.Event) error {
	// Get the task
	var task tasksv1.Task
	err := e.Object.UnmarshalTo(&task)
	if err != nil {
		return err
	}

	taskID := task.GetMeta().GetName()

	// Get current lease
	lease, err := c.clientset.LeaseV1().Get(ctx, task.GetMeta().GetName())
	if errs.IgnoreNotFound(err) != nil {
		c.logger.Error("error getting lease", "error", "task", taskID)
		return err
	}

	currentNodeID := lease.GetConfig().GetNodeId()

	match, err := c.hasMatchingNodes(ctx, &task)
	if err != nil {
		c.logger.Debug("error handling unschedulable task", "error", err, "task", taskID)
		return err
	}

	// Task has no where to go, release lock and updates status
	if !match {
		c.logger.Debug("no nodes matches task's nodeSelector", "task", taskID, "selector", task.GetConfig().GetNodeSelector())
		if err := c.clientset.LeaseV1().Release(ctx, taskID, currentNodeID); err != nil {
			c.logger.Error("error releasing lease for task", "error", err, "task", taskID)
			return err
		}
		if err := c.clientset.TaskV1().Status().Update(ctx, taskID, &tasksv1.Status{
			Phase:  wrapperspb.String(consts.ERRSCHEDULING),
			Reason: wrapperspb.String("no nodes matches node selector"),
		}, "phase", "reason"); err != nil {
			c.logger.Error("error setting task status", "error", err, "task", taskID)
			return err
		}
	}

	return nil
}

func (c *Controller) onTaskCreate(ctx context.Context, e *eventsv1.Event) error {
	// Get the task
	var task tasksv1.Task
	err := e.Object.UnmarshalTo(&task)
	if err != nil {
		return err
	}

	taskID := task.GetMeta().GetName()

	match, err := c.hasMatchingNodes(ctx, &task)
	if err != nil {
		c.logger.Debug("error handling unschedulable task", "error", err, "task", taskID)
		return err
	}

	// Task has no where to go, release lock and updates status
	if !match {
		c.logger.Debug("no nodes matches task's nodeSelector", "task", taskID, "selector", task.GetConfig().GetNodeSelector())
		if err := c.clientset.TaskV1().Status().Update(ctx, taskID, &tasksv1.Status{
			Phase:  wrapperspb.String(consts.ERRSCHEDULING),
			Reason: wrapperspb.String("no nodes matches node selector"),
		}, "phase", "reason"); err != nil {
			c.logger.Error("error setting task status", "error", err, "task", taskID)
			return err
		}
	}

	// Find a node fit for the task using a scheduler
	n, err := c.scheduler.Schedule(ctx, &task)
	if err != nil {
		return err
	}

	// Update task status
	_ = c.clientset.TaskV1().Status().Update(
		ctx,
		task.GetMeta().GetName(),
		&tasksv1.Status{
			Phase: wrapperspb.String("scheduled"),
			Node:  wrapperspb.String(n.GetMeta().GetName()),
		},
		"phase", "node")

	// Publish start event
	err = c.exchange.Publish(ctx, events.NewEvent(events.TaskStart, &task))
	if err != nil {
		return err
	}
	return nil
}

func (c *Controller) onNodeJoin(ctx context.Context, e *eventsv1.Event) error {
	// Get the node§
	var node nodesv1.Node
	err := e.Object.UnmarshalTo(&node)
	if err != nil {
		return err
	}

	c.logger.Debug("emitting task start", "task", node.GetMeta().GetName())

	tasks, err := c.clientset.TaskV1().List(ctx)
	if err != nil {
		return nil
	}

	for _, task := range tasks {
		l, err := c.clientset.LeaseV1().Get(ctx, task.GetMeta().GetName())
		if err != nil {
			return err
		}

		// If lease expired means that task should be rescheduled
		if !time.Now().Before(l.GetConfig().GetExpiresAt().AsTime().Add(time.Second * 10)) {
			c.logger.Debug("emitting task start", "task", task.GetMeta().GetName())
			return c.exchange.Forward(ctx, events.NewEvent(events.TaskStart, task))
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

func (c *Controller) onNodeLabelsChange(ctx context.Context, e *eventsv1.Event) error {
	var node nodesv1.Node
	if err := e.Object.UnmarshalTo(&node); err != nil {
		return err
	}

	tasks, err := c.clientset.TaskV1().List(ctx)
	if err != nil {
		return err
	}

	for _, task := range tasks {

		taskID := task.GetMeta().GetName()

		// Skip tasks without node selector
		if task.GetConfig().GetNodeSelector() == nil || len(task.GetConfig().GetNodeSelector()) == 0 {
			c.logger.Debug("skipping because task has no node selector", "task", taskID)
			continue
		}

		// Get current lease
		lease, err := c.clientset.LeaseV1().Get(ctx, task.GetMeta().GetName())
		if errs.IgnoreNotFound(err) != nil {
			c.logger.Error("error getting lease", "error", "task", taskID)
			continue
		}

		currentNodeID := lease.GetConfig().GetNodeId()

		match, err := c.hasMatchingNodes(ctx, task)
		if err != nil {
			c.logger.Debug("error handling unschedulable task", "error", err, "task", taskID)
			return err
		}

		// Task has no where to go, release lock and updates status
		if !match {
			c.logger.Debug("no nodes matches task's nodeSelector", "task", taskID, "selector", task.GetConfig().GetNodeSelector())
			if err := c.clientset.LeaseV1().Release(ctx, taskID, currentNodeID); err != nil {
				c.logger.Error("error releasing lease for task", "error", err, "task", taskID)
				return err
			}
			if err := c.clientset.TaskV1().Status().Update(ctx, taskID, &tasksv1.Status{
				Phase:  wrapperspb.String(consts.ERRSCHEDULING),
				Reason: wrapperspb.String("no nodes matches node selector"),
			}, "phase", "reason"); err != nil {
				c.logger.Error("error setting task status", "error", err, "task", taskID)
				return err
			}
		}

		// Skip if task is running on another node
		if currentNodeID != node.GetMeta().GetName() {
			continue
		}

		// Check if task still matches THIS node
		selector := labels.NewCompositeSelectorFromMap(task.GetConfig().GetNodeSelector())
		if !selector.Matches(node.GetMeta().GetLabels()) {

			// Task no longer matches - reorganize!
			if err := c.clientset.LeaseV1().Release(ctx, taskID, currentNodeID); err != nil {
				c.logger.Error("error releasing lease for task", "error", err, "task", taskID)
				return err
			}

			if err := c.clientset.TaskV1().Status().Update(ctx, taskID, &tasksv1.Status{
				Phase: wrapperspb.String(consts.PHASESCHEDULING),
			}, "phase"); err != nil {
				c.logger.Error("error setting task status", "error", err, "task", taskID)
			}

			if err := c.exchange.Forward(ctx, events.NewEvent(events.TaskStart, task)); err != nil {
				c.logger.Error("error forwarding task start event", "error", err, "task", taskID)
				return err
			}
		}
	}
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
		events.NodePatch)

	// Setup Handlers
	c.clientset.EventV1().On(events.TaskCreate, c.handleErrors(c.onTaskCreate))
	c.clientset.EventV1().On(events.TaskUpdate, c.handleErrors(c.onTaskUpdate))
	c.clientset.EventV1().On(events.NodeConnect, c.handleErrors(c.onNodeJoin))

	// NEW handlers
	c.clientset.EventV1().On(events.NodeUpdate, c.handleErrors(c.onNodeLabelsChange))
	c.clientset.EventV1().On(events.NodePatch, c.handleErrors(c.onNodeLabelsChange))

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
