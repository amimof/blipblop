package nodecontroller

import (
	"context"
	"fmt"

	eventsv1 "github.com/amimof/voiyd/api/services/events/v1"
	tasksv1 "github.com/amimof/voiyd/api/services/tasks/v1"
	"github.com/amimof/voiyd/pkg/consts"
	cevents "github.com/containerd/containerd/api/events"
	"google.golang.org/protobuf/types/known/wrapperspb"
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

	nodeName := c.node.GetMeta().GetName()

	lease, err := c.clientset.LeaseV1().Get(ctx, tname)
	if err != nil {
		return err
	}

	// Only proceed if task is owned by us
	if lease.GetConfig().GetNodeId() == c.node.GetMeta().GetName() {
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
	if lease.GetConfig().GetNodeId() == c.node.GetMeta().GetName() {
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
	if lease.GetConfig().GetNodeId() == c.node.GetMeta().GetName() {

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
