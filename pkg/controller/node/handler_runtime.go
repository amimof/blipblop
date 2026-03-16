package nodecontroller

import (
	"context"
	"fmt"
	"strconv"

	eventsv1 "github.com/amimof/voiyd/api/services/events/v1"
	"github.com/amimof/voiyd/pkg/condition"
	"github.com/amimof/voiyd/pkg/errs"
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

	task, err := c.clientset.TaskV1().Get(ctx, tname)
	if err != nil {
		return err
	}

	node, err := c.getNode(ctx)
	if err != nil {
		return err
	}

	c.logger.Info("received task start event from runtime", "task", e.GetContainerID(), "pid", e.GetPid())

	report := condition.NewForResource(task).As(node.GetMeta().GetUid())

	md := map[string]string{
		"pid":       strconv.Itoa(int(e.GetPid())),
		"id":        e.GetContainerID(),
		"node_uid":  node.GetMeta().GetUid(),
		"node_name": node.GetMeta().GetName(),
	}

	report.
		Type(condition.TaskScheduled).
		WithMetadata(md).
		True(condition.ReasonScheduled)

	c.Report(report.Report())

	report.
		Type(condition.TaskReady).
		WithMetadata(md).
		True(condition.ReasonRunning)

	c.Report(report.Report())

	_ = c.clientset.TaskV1().Status().SetPid(ctx, task.GetMeta().GetUid(), e.GetPid())
	_ = c.clientset.TaskV1().Status().SetID(ctx, task.GetMeta().GetUid(), e.GetContainerID())

	return nil
}

func (c *Controller) onRuntimeTaskExit(ctx context.Context, obj *eventsv1.Event) error {
	var e cevents.TaskExit
	err := obj.GetObject().UnmarshalTo(&e)
	if err != nil {
		return err
	}

	pid, err := c.runtime.Pid(ctx, e.GetContainerID())
	if err != nil {
		return err
	}

	err = c.netmanager.Detach(ctx, e.GetContainerID(), pid)
	if errs.IgnoreNotFound(err) != nil {
		return err
	}

	task, err := c.clientset.TaskV1().Get(ctx, e.GetContainerID())
	if err != nil {
		return err
	}

	node, err := c.getNode(ctx)
	if err != nil {
		return err
	}

	c.logger.Info("received task exit event from runtime", "exitCode", e.GetExitStatus(), "pid", e.GetPid(), "exitedAt", e.GetExitedAt())

	report := condition.NewForResource(task).As(node.GetMeta().GetUid())

	exitStatus := fmt.Sprintf("exit status %d", e.GetExitStatus())

	md := map[string]string{"exit_status": exitStatus}

	taskReport := report.
		Type(condition.TaskReady).
		WithMetadata(md).
		False(condition.ReasonStopped, exitStatus)

	c.Report(taskReport)

	// Only update the phase if this exit was not triggered by an intentional stop/kill.
	// When stopTask/killTask is in progress it owns the final phase update.
	// During startTask re-provisioning, the exit is transient and should be suppressed.
	c.operationsMu.RLock()
	_, inProgress := c.operations[task.GetMeta().GetUid()]
	c.operationsMu.RUnlock()

	if !inProgress {
		_ = c.clientset.TaskV1().Status().SetPhase(ctx, task.GetMeta().GetUid(), string(condition.ReasonStopped))
		_ = c.clientset.TaskV1().Status().SetReason(ctx, task.GetMeta().GetUid(), exitStatus)
	}

	return nil
}

func (c *Controller) onRuntimeTaskDelete(ctx context.Context, obj *eventsv1.Event) error {
	var e cevents.TaskDelete
	err := obj.GetObject().UnmarshalTo(&e)
	if err != nil {
		return err
	}

	task, err := c.clientset.TaskV1().Get(ctx, e.GetContainerID())
	if err != nil {
		return err
	}

	node, err := c.getNode(ctx)
	if err != nil {
		return err
	}

	c.logger.Info("received task delete event from runtime", "task", e.GetContainerID(), "pid", e.GetPid())

	report := condition.NewForResource(task).As(node.GetMeta().GetUid())
	md := map[string]string{
		"id":  "",
		"pid": "",
	}
	taskReport := report.
		Type(condition.TaskReady).
		WithMetadata(md).
		False(condition.ReasonStopped)

	c.Report(taskReport)

	// Only set phase to "Deleted" when the deletion was unexpected (not triggered
	// by an intentional stop/kill or startTask re-provisioning). If stopTask or
	// killTask is in progress they own the final phase and will set it to "Stopped".
	// During startTask the delete is transient; it will set "Running" on success.
	c.operationsMu.RLock()
	_, inProgress := c.operations[task.GetMeta().GetUid()]
	c.operationsMu.RUnlock()

	if !inProgress {
		_ = c.clientset.TaskV1().Status().SetPhase(ctx, task.GetMeta().GetUid(), string(condition.ReasonDeleted))
	}

	return nil
}
