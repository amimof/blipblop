package nodecontroller

import (
	"context"

	eventsv1 "github.com/amimof/voiyd/api/services/events/v1"
	tasksv1 "github.com/amimof/voiyd/api/services/tasks/v1"
	"github.com/amimof/voiyd/pkg/consts"
	errs "github.com/amimof/voiyd/pkg/errors"
	"github.com/amimof/voiyd/pkg/events"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type TaskHandlerFunc func(context.Context, *tasksv1.Task) error

func (c *Controller) handleTaskNodeSelector(h TaskHandlerFunc) TaskHandlerFunc {
	return func(ctx context.Context, task *tasksv1.Task) error {
		nodeID := c.node.GetMeta().GetName()

		if !c.isNodeSelected(ctx, nodeID, task) {
			c.logger.Debug("discarded due to label mismatch", "task", task.GetMeta().GetName())
			return nil
		}

		err := h(ctx, task)
		if err != nil {
			return err
		}

		return nil
	}
}

func (c *Controller) handleTask(h TaskHandlerFunc) events.HandlerFunc {
	return func(ctx context.Context, ev *eventsv1.Event) error {
		var task tasksv1.Task
		err := ev.GetObject().UnmarshalTo(&task)
		if err != nil {
			return err
		}

		err = h(ctx, &task)
		if err != nil {
			return err
		}

		return nil
	}
}

func (c *Controller) onTaskStart(ctx context.Context, task *tasksv1.Task) error {
	ctx, span := c.tracer.Start(ctx, "controller.node.OnTaskStart")
	defer span.End()

	c.logger.Debug("controller received task start", "name", task.GetMeta().GetName())

	return c.startTask(ctx, task)
}

func (c *Controller) onTaskStop(ctx context.Context, task *tasksv1.Task) error {
	ctx, span := c.tracer.Start(ctx, "controller.node.OnTaskStop")
	defer span.End()

	c.logger.Info("controller received task stop", "name", task.GetMeta().GetName())

	return c.stopTask(ctx, task)
}

func (c *Controller) onTaskDelete(ctx context.Context, task *tasksv1.Task) error {
	ctx, span := c.tracer.Start(ctx, "controller.node.OnTaskDelete")
	defer span.End()

	taskID := task.GetMeta().GetName()
	c.logger.Info("controller received task delete", "name", task.GetMeta().GetName())

	_ = c.clientset.TaskV1().Status().Update(ctx, taskID, &tasksv1.Status{Phase: wrapperspb.String(consts.PHASESTOPPING)}, "phase")
	err := c.runtime.Delete(ctx, task)
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

func (c *Controller) onTaskUpdate(ctx context.Context, task *tasksv1.Task) error {
	ctx, span := c.tracer.Start(ctx, "controller.node.OnTaskUpdate")
	defer span.End()

	err := c.stopTask(ctx, task)
	if errs.IgnoreNotFound(err) != nil {
		return err
	}

	// err = c.onTaskStart(ctx, e)
	err = c.startTask(ctx, task)
	if err != nil {
		return err
	}
	return nil
}

func (c *Controller) onTaskKill(ctx context.Context, task *tasksv1.Task) error {
	ctx, span := c.tracer.Start(ctx, "controller.node.OnTaskKill")
	defer span.End()

	taskID := task.GetMeta().GetName()
	c.logger.Info("controller received task kill", "name", taskID)

	_ = c.clientset.TaskV1().Status().Update(ctx, taskID, &tasksv1.Status{Phase: wrapperspb.String(consts.PHASESTOPPING)}, "phase")
	err := c.runtime.Kill(ctx, task)
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

	return nil
}
