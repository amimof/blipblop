package nodecontroller

import (
	"context"
	"errors"

	tasksv1 "github.com/amimof/voiyd/api/services/tasks/v1"
	"github.com/amimof/voiyd/pkg/condition"
	errs "github.com/amimof/voiyd/pkg/errors"
	"github.com/amimof/voiyd/pkg/events"
)

func (c *Controller) handleNodeSelector(h events.TaskHandlerFunc) events.TaskHandlerFunc {
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

func (c *Controller) updateTask(ctx context.Context, task *tasksv1.Task) error {
	ctx, span := c.tracer.Start(ctx, "controller.node.OnTaskUpdate")
	defer span.End()

	err := c.stopTask(ctx, task)
	if errs.IgnoreNotFound(err) != nil {
		return err
	}

	err = c.startTask(ctx, task)
	if err != nil {
		return err
	}
	return nil
}

func (c *Controller) killTask(ctx context.Context, task *tasksv1.Task) error {
	ctx, span := c.tracer.Start(ctx, "controller.node.OnTaskKill")
	defer span.End()

	taskID := task.GetMeta().GetName()
	nodeID := c.node.GetMeta().GetName()

	// Release lease
	defer func() {
		err := c.clientset.LeaseV1().Release(ctx, taskID, nodeID)
		if err != nil {
			c.logger.Warn("unable to release lease", "error", err, "task", taskID, "nodeID", nodeID)
		}
	}()

	// Run cleanup early while netns still exists.
	// This will allow the CNI plugin to remove networks without leaking.
	err := c.cleanup(ctx, task)
	if err != nil {
		return err
	}

	// Stop the task
	err = c.stopTask(ctx, task)
	if errs.IgnoreNotFound(err) != nil {
		return err
	}

	// Remove any previous tasks
	err = c.runtime.Kill(ctx, task)
	if err != nil {
		return err
	}

	// Detach volumes
	return c.detachMounts(ctx, task)
}

func (c *Controller) stopTask(ctx context.Context, task *tasksv1.Task) error {
	ctx, span := c.tracer.Start(ctx, "controller.node.OnTaskStop")
	defer span.End()

	taskID := task.GetMeta().GetName()
	nodeID := c.node.GetMeta().GetName()

	// Release lease
	defer func() {
		err := c.clientset.LeaseV1().Release(ctx, taskID, nodeID)
		if err != nil {
			c.logger.Warn("unable to release lease", "error", err, "task", taskID, "nodeID", nodeID)
		}
	}()

	// Run cleanup early while netns still exists.
	// This will allow the CNI plugin to remove networks without leaking.
	err := c.cleanup(ctx, task)
	if err != nil {
		return err
	}

	// Stop the task
	err = c.runtime.Stop(ctx, task)
	if errs.IgnoreNotFound(err) != nil {
		return err
	}

	// Remove any previous tasks
	err = c.deleteTask(ctx, task)
	if err != nil {
		return err
	}

	// Detach volumes
	return c.detachMounts(ctx, task)
}

func (c *Controller) acquireLease(ctx context.Context, task *tasksv1.Task) error {
	taskID := task.GetMeta().GetName()
	nodeID := c.node.GetMeta().GetName()

	ttl, expired, err := c.clientset.LeaseV1().Acquire(ctx, taskID, nodeID)
	if err != nil {
		c.logger.Error("failed to acquire lease", "error", err, "task", taskID, "nodeID", nodeID)
		return err
	}

	// Release if task can't be provisioned
	defer func() {
		if err != nil {
			err = c.clientset.LeaseV1().Release(ctx, taskID, nodeID)
			if err != nil {
				c.logger.Warn("unable to release lease", "task", taskID, "node", nodeID)
			}
		}
	}()

	if !expired {
		c.logger.Warn("lease held by another node", "task", taskID)
		return errors.New("lease held by another another")
	}

	c.logger.Info("acquired lease for task", "task", taskID, "node", nodeID, "ttl", ttl)
	return nil
}

func (c *Controller) cleanup(ctx context.Context, task *tasksv1.Task) error {
	taskID := task.GetMeta().GetName()
	nodeID := c.node.GetMeta().GetName()
	reporter := condition.NewReportFor(task, nodeID)

	// Run cleanup early while netns still exists.
	// This will allow the CNI plugin to remove networks without leaking.
	_ = c.runtime.Cleanup(ctx, taskID)

	// Remove any previous tasks ignoring any errors
	_ = c.clientset.EventV1().Report(ctx, reporter.Type(condition.TaskReady).InProgress(condition.ReasonDeleting, ""))
	err := c.runtime.Delete(ctx, task)
	if err != nil {
		_ = c.clientset.EventV1().Report(ctx, reporter.Type(condition.TaskReady).False(condition.ReasonDeleteFailed, err.Error()))
		return err
	}
	return c.clientset.EventV1().Report(ctx, reporter.Type(condition.TaskReady).InProgress(condition.ReasonDeleted, ""))
}

func (c *Controller) deleteTask(ctx context.Context, task *tasksv1.Task) error {
	ctx, span := c.tracer.Start(ctx, "controller.node.OnTaskDelete")
	defer span.End()

	nodeID := c.node.GetMeta().GetName()
	reporter := condition.NewReportFor(task, nodeID)

	// Remove any previous tasks ignoring any errors
	_ = c.clientset.EventV1().Report(ctx, reporter.Type(condition.TaskReady).InProgress(condition.ReasonDeleting, ""))
	err := c.runtime.Delete(ctx, task)
	if err != nil {
		_ = c.clientset.EventV1().Report(ctx, reporter.Type(condition.TaskReady).False(condition.ReasonDeleteFailed, err.Error()))
		return err
	}

	return nil
}

func (c *Controller) attachMounts(ctx context.Context, task *tasksv1.Task) error {
	nodeID := c.node.GetMeta().GetName()
	reporter := condition.NewReportFor(task, nodeID)

	// Prepare volumes/mounts
	_ = c.clientset.EventV1().Report(ctx, reporter.Type(condition.VolumeReady).InProgress(condition.ReasonAttaching, ""))
	if err := c.attacher.PrepareMounts(ctx, c.node, task); err != nil {
		_ = c.clientset.EventV1().Report(ctx, reporter.Type(condition.VolumeReady).False(condition.ReasonAttachFailed, err.Error()))
		return err
	}

	return c.clientset.EventV1().Report(ctx, reporter.Type(condition.VolumeReady).True(condition.ReasonAttached, ""))
}

func (c *Controller) detachMounts(ctx context.Context, task *tasksv1.Task) error {
	nodeID := c.node.GetMeta().GetName()
	reporter := condition.NewReportFor(task, nodeID)

	// Prepare volumes/mounts
	_ = c.clientset.EventV1().Report(ctx, reporter.Type(condition.VolumeReady).InProgress(condition.ReasonDetaching, ""))
	if err := c.attacher.Detach(ctx, c.node, task); err != nil {
		_ = c.clientset.EventV1().Report(ctx, reporter.Type(condition.VolumeReady).False(condition.ReasonDetachFailed, err.Error()))
		return err
	}

	return c.clientset.EventV1().Report(ctx, reporter.Type(condition.VolumeReady).True(condition.ReasonDetached, ""))
}

func (c *Controller) pullImage(ctx context.Context, task *tasksv1.Task) error {
	nodeID := c.node.GetMeta().GetName()
	reporter := condition.NewReportFor(task, nodeID)

	// Pull image
	_ = c.clientset.EventV1().Report(ctx, reporter.Type(condition.ImageReady).InProgress(condition.ReasonPulling, ""))
	err := c.runtime.Pull(ctx, task)
	if err != nil {
		_ = c.clientset.EventV1().Report(ctx, reporter.Type(condition.ImageReady).False(condition.ReasonPullFailed, err.Error()))
		return err
	}
	return c.clientset.EventV1().Report(ctx, reporter.Type(condition.ImageReady).True(condition.ReasonPulled, ""))
}

func (c *Controller) startTask(ctx context.Context, task *tasksv1.Task) error {
	ctx, span := c.tracer.Start(ctx, "controller.node.OnTaskStart")
	defer span.End()

	err := c.acquireLease(ctx, task)
	if err != nil {
		return err
	}

	// Run cleanup early while netns still exists.
	// This will allow the CNI plugin to remove networks without leaking.
	err = c.cleanup(ctx, task)
	if err != nil {
		return err
	}

	err = c.deleteTask(ctx, task)
	if err != nil {
		return err
	}

	err = c.attachMounts(ctx, task)
	if err != nil {
		return err
	}

	err = c.pullImage(ctx, task)
	if err != nil {
		return err
	}

	return c.runtime.Run(ctx, task)
}
