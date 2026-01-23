package nodecontroller

import (
	"context"
	"errors"
	"fmt"

	nodesv1 "github.com/amimof/voiyd/api/services/nodes/v1"
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

	// Remove any previous tasks
	err := c.runtime.Kill(ctx, task)
	if err != nil {
		return err
	}

	err = c.detachMounts(ctx, task)
	if err != nil {
		return err
	}

	// Detach volumes
	return c.deleteTask(ctx, task)
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

	// Detach volumes
	err := c.detachMounts(ctx, task)
	if err != nil {
		return err
	}

	// Stop the task
	err = c.runtime.Stop(ctx, task)
	if errs.IgnoreNotFound(err) != nil {
		return fmt.Errorf("error stopping task AMIRMIR: %v", err)
	}

	// Remove any previous tasks
	return c.deleteTask(ctx, task)
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

func (c *Controller) deleteTask(ctx context.Context, task *tasksv1.Task) error {
	ctx, span := c.tracer.Start(ctx, "controller.node.OnTaskDelete")
	defer span.End()

	taskID := task.GetMeta().GetName()
	report := condition.NewForResource(task)

	// Run cleanup early while netns still exists.
	// This will allow the CNI plugin to remove networks without leaking.
	_ = c.runtime.Cleanup(ctx, taskID)

	// Remove any previous tasks ignoring any errors

	report.
		Type(condition.TaskReady).
		False(condition.ReasonDeleting)

	_ = c.clientset.TaskV1().Condition(ctx, report.Report())
	err := c.runtime.Delete(ctx, task)
	if err != nil {
		report.
			Type(condition.TaskReady).
			False(condition.ReasonDeleteFailed, err.Error())
		_ = c.clientset.TaskV1().Condition(ctx, report.Report())
		return err
	}

	report.
		Type(condition.TaskScheduled).
		WithMetadata(map[string]string{"node": ""}).
		False(condition.ReasonStopped)

	_ = c.clientset.TaskV1().Condition(ctx, report.Report())

	report.
		Type(condition.TaskReady).
		WithMetadata(map[string]string{"pid": "", "id": ""}).
		False(condition.ReasonStopped)

	return c.clientset.TaskV1().Condition(ctx, report.Report())
}

func (c *Controller) attachMounts(ctx context.Context, task *tasksv1.Task) error {
	report := condition.NewForResource(task)

	// Prepare volumes/mounts
	report.
		Type(condition.VolumeReady).
		False(condition.ReasonAttaching)
	_ = c.clientset.TaskV1().Condition(ctx, report.Report())

	if err := c.attacher.PrepareMounts(ctx, c.node, task); err != nil {
		report.
			Type(condition.VolumeReady).
			False(condition.ReasonAttachFailed, err.Error())
		_ = c.clientset.TaskV1().Condition(ctx, report.Report())
		return err
	}

	report.
		Type(condition.VolumeReady).
		True(condition.ReasonAttached)

	return c.clientset.TaskV1().Condition(ctx, report.Report())
}

func (c *Controller) detachMounts(ctx context.Context, task *tasksv1.Task) error {
	report := condition.NewForResource(task)

	// Prepare volumes/mounts
	report.
		Type(condition.VolumeReady).
		False(condition.ReasonDetaching)
	_ = c.clientset.TaskV1().Condition(ctx, report.Report())

	if err := c.attacher.Detach(ctx, c.node, task); err != nil {
		report.
			Type(condition.ImageReady).
			False(condition.ReasonPullFailed)
		_ = c.clientset.TaskV1().Condition(ctx, report.Report())
		return err
	}

	report.
		Type(condition.VolumeReady).
		False(condition.ReasonDetached)
	return c.clientset.TaskV1().Condition(ctx, report.Report())
}

func (c *Controller) pullImage(ctx context.Context, task *tasksv1.Task) error {
	report := condition.NewForResource(task)

	// Pull image
	report.
		Type(condition.ImageReady).
		False(condition.ReasonPulling)

	_ = c.clientset.TaskV1().Condition(ctx, report.Report())

	err := c.runtime.Pull(ctx, task)
	if err != nil {
		report.
			Type(condition.ImageReady).
			False(condition.ReasonPullFailed, err.Error())
		_ = c.clientset.TaskV1().Condition(ctx, report.Report())
		return err
	}

	report.
		Type(condition.ImageReady).
		True(condition.ReasonPulled)

	return c.clientset.TaskV1().Condition(ctx, report.Report())
}

func (c *Controller) onSchedule(ctx context.Context, task *tasksv1.Task, _ *nodesv1.Node) error {
	return c.handleNodeSelector(c.startTask)(ctx, task)
}

func (c *Controller) startTask(ctx context.Context, task *tasksv1.Task) error {
	ctx, span := c.tracer.Start(ctx, "controller.node.OnTaskStart")
	defer span.End()

	err := c.acquireLease(ctx, task)
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
