package nodecontroller

import (
	"context"
	"fmt"
	"strconv"

	eventsv1 "github.com/amimof/voiyd/api/services/events/v1"
	"github.com/amimof/voiyd/pkg/condition"
	cevents "github.com/containerd/containerd/api/events"
)

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

	lease, err := c.clientset.LeaseV1().Get(ctx, tname)
	if err != nil {
		return err
	}

	task, err := c.clientset.TaskV1().Get(ctx, tname)
	if err != nil {
		return err
	}

	// Only proceed if task is owned by us
	if lease.GetConfig().GetNodeId() == c.node.GetMeta().GetName() {
		c.logger.Info("received task start event from runtime", "task", e.GetContainerID(), "pid", e.GetPid())

		nodeID := c.node.GetMeta().GetName()
		taskID := task.GetMeta().GetName()
		taskGen := task.GetMeta().GetRevision()
		reporter := condition.NewReport(taskID, nodeID, int64(taskGen))

		return c.clientset.EventV1().Report(ctx, reporter.Type(condition.TaskReady).WithMetadata(map[string]string{"pid": strconv.Itoa(int(e.GetPid())), "id": e.GetContainerID()}).True(condition.ReasonRunning, ""))
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

	task, err := c.clientset.TaskV1().Get(ctx, tname)
	if err != nil {
		return err
	}

	// Only proceed if task is owned by us
	c.logger.Info("received task exit event from runtime", "exitCode", e.GetExitStatus(), "pid", e.GetPid(), "exitedAt", e.GetExitedAt())

	nodeID := c.node.GetMeta().GetName()
	taskID := task.GetMeta().GetName()
	taskGen := task.GetMeta().GetRevision()
	reporter := condition.NewReport(taskID, nodeID, int64(taskGen))
	status := ""

	if e.GetExitStatus() > 0 {
		status = fmt.Sprintf("exit status %d", e.GetExitStatus())
	}

	return c.clientset.EventV1().Report(ctx, reporter.Type(condition.TaskReady).WithMetadata(map[string]string{"exit_status": status}).False(condition.ReasonStopped, status))
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

	task, err := c.clientset.TaskV1().Get(ctx, tname)
	if err != nil {
		return err
	}

	// Only proceed if task is owned by us
	c.logger.Info("received task delete event from runtime", "task", e.GetContainerID(), "pid", e.GetPid())

	nodeID := c.node.GetMeta().GetName()
	taskID := task.GetMeta().GetName()
	taskGen := task.GetMeta().GetRevision()
	reporter := condition.NewReport(taskID, nodeID, int64(taskGen))

	return c.clientset.EventV1().Report(ctx, reporter.Type(condition.TaskReady).False(condition.ReasonStopped, "Task is stopped"))
}
