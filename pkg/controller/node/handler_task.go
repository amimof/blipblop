package nodecontroller

import (
	"context"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/containerd/errdefs"
	gocni "github.com/containerd/go-cni"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/amimof/voiyd/pkg/condition"
	"github.com/amimof/voiyd/pkg/errs"
	"github.com/amimof/voiyd/pkg/networking"
	"github.com/amimof/voiyd/pkg/queue"
	"github.com/amimof/voiyd/pkg/runtime"

	nodesv1 "github.com/amimof/voiyd/api/services/nodes/v1"
	tasksv1 "github.com/amimof/voiyd/api/services/tasks/v1"
	typesv1 "github.com/amimof/voiyd/api/types/v1"
)

// Uint64ToBytes converts the given uint64 value to slice of bytes.
func Uint64ToBytes(val uint64) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, val)
	return b
}

// processScheduleTask enqueues a task for async processing by the workpool
func (c *Controller) processScheduleTask(ctx context.Context, task *tasksv1.Task, node *nodesv1.Node) error {
	taskUID := task.GetMeta().GetUid()
	nodeUID := task.GetMeta().GetUid()

	c.logger.Debug("enqueuing task for scheduling",
		"task", taskUID,
		"node", nodeUID)

	err := c.acquireLease(ctx, task)
	if err != nil {
		if errs.IsConflict(err) {
			if err := c.renewLease(ctx, task); err != nil {
				// Task will be garbage collected in renew lease-loop
				c.logger.Debug("error getting refresh token for task", "error", err, "task", taskUID)
				return nil
			}
		}
		c.logger.Warn("couldnt acquire lease", " error", err)
		return nil
	}

	return c.queue.Enqueue(ctx, &queue.QueueItem{
		Task:       task,
		EnqueuedAt: time.Now(),
		RetryCount: 0,
		Handler:    c.startTask,
	})
}

// processScheduleTask enqueues a task for async processing by the workpool
func (c *Controller) processStopTask(ctx context.Context, task *tasksv1.Task) error {
	c.logger.Debug("enqueuing stop task",
		"task", task.GetMeta().GetName())

	return c.queue.Enqueue(ctx, &queue.QueueItem{
		Task:       task,
		EnqueuedAt: time.Now(),
		RetryCount: 0,
		Handler:    c.stopTask,
	})
}

// processScheduleTask enqueues a task for async processing by the workpool
func (c *Controller) processKillTask(ctx context.Context, task *tasksv1.Task) error {
	c.logger.Debug("enqueuing kill task",
		"task", task.GetMeta().GetName())

	return c.queue.Enqueue(ctx, &queue.QueueItem{
		Task:       task,
		EnqueuedAt: time.Now(),
		RetryCount: 0,
		Handler:    c.killTask,
	})
}

func (c *Controller) killTask(ctx context.Context, task *tasksv1.Task) error {
	ctx, span := c.tracer.Start(ctx, "controller.node.OnTaskKill")
	defer span.End()

	node, err := c.getNode(ctx)
	if err != nil {
		return err
	}

	taskID := task.GetMeta().GetUid()
	nodeID := node.GetMeta().GetUid()

	// Release lease
	defer func() {
		refreshToken, err := c.tokenStore.Load(taskID)
		if err != nil {
			return
		}
		err = c.clientset.LeaseV1().Release(ctx, taskID, nodeID, string(refreshToken))
		if err != nil {
			c.logger.Warn("unable to release lease", "error", err, "task", taskID, "nodeID", nodeID)
		}
	}()

	report := condition.NewForResource(task).As(c.node.GetMeta().GetUid())

	// Detach network
	err = c.detachNetwork(ctx, task, report)
	if errs.IgnoreNotFound(err) != nil {
		return err
	}

	// Remove any previous tasks
	err = c.runtime.Kill(ctx, task)
	if err != nil {
		return err
	}

	err = c.detachMounts(ctx, task, report)
	if !errdefs.IsNotFound(err) {
		return err
	}

	// Detach volumes
	return c.deleteTask(ctx, task, report)
}

func (c *Controller) stopTask(ctx context.Context, task *tasksv1.Task) error {
	ctx, span := c.tracer.Start(ctx, "controller.node.OnTaskStop")
	defer span.End()

	node, err := c.getNode(ctx)
	if err != nil {
		return err
	}

	taskID := task.GetMeta().GetUid()
	nodeID := node.GetMeta().GetUid()
	report := condition.NewForResource(task).As(nodeID)

	err = c.detachNetwork(ctx, task, report)
	if errs.IgnoreNotFound(err) != nil {
		return err
	}

	// Release lease
	defer func() {
		refreshToken, err := c.tokenStore.Load(taskID)
		if err != nil {
			return
		}
		err = c.clientset.LeaseV1().Release(ctx, taskID, nodeID, string(refreshToken))
		if err != nil {
			c.logger.Warn("unable to release lease", "error", err, "task", taskID, "nodeID", nodeID)
		}

		c.mu.Lock()
		delete(c.epochs, taskID)
		c.mu.Unlock()

		_ = c.tokenStore.Delete(taskID)
	}()

	err = c.detachNetwork(ctx, task, report)
	if errs.IgnoreNotFound(err) != nil {
		return err
	}

	err = c.detachMounts(ctx, task, report)
	if errs.IgnoreNotFound(err) != nil {
		return nil
	}

	// Stop the task
	err = c.runtime.Stop(ctx, task)
	if errs.IgnoreNotFound(err) != nil {
		return err
	}

	// Remove any previous tasks
	err = c.deleteTask(ctx, task, report)
	if err != nil {
		return err
	}

	_ = c.clientset.TaskV1().Status().SetPhase(ctx, task.GetMeta().GetUid(), string(condition.ReasonStopped))
	_ = c.clientset.TaskV1().Status().SetReason(ctx, task.GetMeta().GetUid(), "")

	return nil
}

// getNode fetches the node by either node name or node UID from the server. It uses whichever idenfier that exists in the
// local node config but prioritizes node uid's and will fall back to names.
func (c *Controller) getNode(ctx context.Context) (*nodesv1.Node, error) {
	var nodeID string
	if c.node.GetMeta().GetName() != "" {
		nodeID = c.node.GetMeta().GetName()
	}
	if c.node.GetMeta().GetUid() != "" {
		nodeID = c.node.GetMeta().GetUid()
	}
	return c.clientset.NodeV1().Get(ctx, nodeID)
}

func (c *Controller) renewLease(ctx context.Context, task *tasksv1.Task) error {
	node, err := c.getNode(ctx)
	if err != nil {
		return err
	}

	taskID := task.GetMeta().GetUid()
	nodeID := node.GetMeta().GetUid()

	refreshToken, err := c.tokenStore.Load(taskID)
	if err != nil {
		c.logger.Debug("refresh token did not exist in store", "error", err, "task", taskID)
		return nil
	}

	_, newToken, fencing, err := c.clientset.LeaseV1().Renew(ctx, taskID, nodeID, string(refreshToken))
	if err != nil {
		c.logger.Error("failed to renew lease", "error", err, "task", taskID, "nodeID", nodeID)
		return err
	}

	c.mu.Lock()
	c.epochs[taskID] = fencing
	c.mu.Unlock()

	err = c.tokenStore.Save(taskID, []byte(newToken))
	if err != nil {
		return err
	}

	c.logger.Debug("renewed lease for task", "task", taskID, "node", nodeID)
	return nil
}

func (c *Controller) acquireLease(ctx context.Context, task *tasksv1.Task) error {
	node, err := c.getNode(ctx)
	if err != nil {
		return err
	}

	taskID := task.GetMeta().GetUid()
	nodeID := node.GetMeta().GetUid()

	lease, token, fencing, err := c.clientset.LeaseV1().Acquire(ctx, taskID, nodeID)
	if err != nil {
		c.logger.Error("failed to acquire lease", "error", err, "task", taskID, "nodeID", nodeID)
		return err
	}

	c.mu.Lock()
	c.epochs[taskID] = fencing
	c.mu.Unlock()

	// Persist refresh token so we can recover from restarts
	err = c.tokenStore.Save(taskID, []byte(token))
	if err != nil {
		return err
	}

	// Release if task can't be provisioned
	defer func() {
		if err != nil {
			err = c.clientset.LeaseV1().Release(ctx, taskID, nodeID, token)
			if err != nil {
				c.logger.Warn("unable to release lease", "task", taskID, "node", nodeID)
			}
		}
	}()

	c.logger.Info("acquired lease for task", "task", taskID, "node", nodeID, "ttl", lease.GetConfig().GetTtlSeconds())
	return nil
}

func (c *Controller) deleteTask(ctx context.Context, task *tasksv1.Task, report *condition.Report) error {
	ctx, span := c.tracer.Start(ctx, "controller.node.OnTaskDelete")
	defer span.End()

	taskID := task.GetMeta().GetUid()

	// Run cleanup early while netns still exists.
	// This will allow the CNI plugin to remove networks without leaking.
	_ = c.runtime.Cleanup(ctx, taskID)

	// Remove any previous tasks ignoring any errors

	report.
		Type(condition.TaskReady).
		False(condition.ReasonDeleting)

	c.Report(report.Report())
	err := c.runtime.Delete(ctx, task)
	if err != nil {
		if !errs.IsNotFound(err) {
			report.
				Type(condition.TaskReady).
				False(condition.ReasonDeleteFailed, err.Error())
			c.Report(report.Report())

		}
		return err
	}

	report.
		Type(condition.TaskScheduled).
		WithMetadata(map[string]string{"node_name": "", "node_uid": ""}).
		False(condition.ReasonStopped)

	c.Report(report.Report())

	report.
		Type(condition.TaskReady).
		WithMetadata(map[string]string{"pid": "", "id": ""}).
		False(condition.ReasonStopped)

	c.Report(report.Report())
	return nil
}

func (c *Controller) attachMounts(ctx context.Context, task *tasksv1.Task, report *condition.Report) error {
	// Prepare volumes/mounts
	report.
		Type(condition.VolumeReady).
		False(condition.ReasonAttaching)

	c.Report(report.Report())

	if err := c.attacher.PrepareMounts(ctx, c.node, task); err != nil {
		report.
			Type(condition.VolumeReady).
			False(condition.ReasonAttachFailed, err.Error())
		c.Report(report.Report())
		return err
	}

	report.
		Type(condition.VolumeReady).
		True(condition.ReasonAttached, "")

	c.Report(report.Report())
	return nil
}

func (c *Controller) detachMounts(ctx context.Context, task *tasksv1.Task, report *condition.Report) error {
	// Prepare volumes/mounts
	report.
		Type(condition.VolumeReady).
		False(condition.ReasonDetaching)
	c.Report(report.Report())

	if err := c.attacher.Detach(ctx, c.node, task); err != nil {
		if !errs.IsNotFound(err) {
			report.
				Type(condition.ImageReady).
				False(condition.ReasonPullFailed)
			c.Report(report.Report())
		}

		return err
	}

	report.
		Type(condition.VolumeReady).
		False(condition.ReasonDetached)
	c.Report(report.Report())
	return nil
}

func (c *Controller) pullImage(ctx context.Context, task *tasksv1.Task, report *condition.Report) error {
	// Pull image
	report.
		Type(condition.ImageReady).
		False(condition.ReasonPulling)

	c.Report(report.Report())

	err := c.runtime.Pull(ctx, task)
	if err != nil {
		if !errs.IsNotFound(err) {
			report.
				Type(condition.ImageReady).
				False(condition.ReasonPullFailed, err.Error())
			c.Report(report.Report())
		}
		return err
	}

	report.
		Type(condition.ImageReady).
		True(condition.ReasonPulled)

	c.Report(report.Report())
	return nil
}

// Returns a version of task that is comparable by stripping fields that
// should be omitted when comparing with proto.Equal for example
func comparable(task *tasksv1.Task) *tasksv1.Task {
	t := proto.Clone(task).(*tasksv1.Task)
	t.Status = nil
	t.GetMeta().ResourceVersion = 0
	return t
}

func (c *Controller) startTask(ctx context.Context, task *tasksv1.Task) (err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	ctx, span := c.tracer.Start(ctx, "controller.node.OnTaskStart")
	defer span.End()

	defer func() {
		if err != nil {
			_ = c.clientset.TaskV1().Status().Update(
				ctx,
				task.GetMeta().GetUid(),
				&tasksv1.Status{
					Phase:  wrapperspb.String(string(condition.ReasonStartFailed)),
					Reason: wrapperspb.String(err.Error()),
				},
			)
		}
	}()

	t, err := c.runtime.Get(ctx, task.GetMeta().GetUid())
	if errs.IgnoreNotFound(err) != nil {
		return err
	}

	// Skip task provision if no changes are made. Ignore status-fields when comparing
	if t != nil {
		if proto.Equal(comparable(t), comparable(task)) {
			c.logger.Debug("task does not require re-provisioning", "task", task.GetMeta().GetName())
			return nil
		}
	}

	// Build a set of additional labels to pass to the runtime
	labels := make(map[string]string)
	if epoch, ok := c.epochs[task.GetMeta().GetUid()]; ok {
		labels["voiyd.io/epoch"] = fmt.Sprintf("%d", epoch)
	}

	report := condition.NewForResource(task).As(c.node.GetMeta().GetUid())

	err = c.detachNetwork(ctx, task, report)
	if errs.IgnoreNotFound(err) != nil {
		return err
	}

	err = c.deleteTask(ctx, task, report)
	if err != nil {
		return err
	}

	err = c.attachMounts(ctx, task, report)
	if err != nil {
		return err
	}

	err = c.pullImage(ctx, task, report)
	if err != nil {
		return err
	}

	err = c.runtime.Run(ctx, task, runtime.WithContainerdLabels(labels))
	if err != nil {
		return err
	}

	err = c.attachNetwork(ctx, task, report)
	if err != nil {
		return err
	}

	_ = c.clientset.TaskV1().Status().SetPhase(ctx, task.GetMeta().GetUid(), string(condition.ReasonRunning))
	_ = c.clientset.TaskV1().Status().SetReason(ctx, task.GetMeta().GetUid(), "")

	return nil
}

func (c *Controller) Report(report *typesv1.ConditionReport) {
	status := report.GetStatus() == typesv1.ConditionStatus_CONDITION_STATUS_TRUE
	c.publisher.Report(report.GetResourceId(), status, report)
}

func (c *Controller) detachNetwork(ctx context.Context, task *tasksv1.Task, report *condition.Report) error {
	c.Report(report.Type(condition.NetworkReady).False(condition.ReasonDetaching))

	pid, err := c.runtime.Pid(ctx, task.GetMeta().GetUid())
	if err != nil {
		if !errs.IsNotFound(err) {
			c.Report(report.Type(condition.NetworkReady).False(condition.ReasonDetachFailed, err.Error()))
		}
		return err
	}

	if pid != 0 {
		pm := networking.ParseCNIPortMappings(task.GetConfig().PortMappings...)
		attachOpts := []gocni.NamespaceOpts{gocni.WithCapabilityPortMap(pm), gocni.WithArgs("IgnoreUnknown", "true")}

		// Delete CNI Network
		err = c.netmanager.Detach(ctx, task.GetMeta().GetUid(), pid, attachOpts...)
		if err != nil {
			if !errs.IsNotFound(err) {
				c.Report(report.Type(condition.NetworkReady).False(condition.ReasonDetachFailed, err.Error()))
			}
			return err
		}

	}

	md := map[string]string{
		"ip_address": "",
		"gateway":    "",
	}

	c.Report(report.Type(condition.NetworkReady).WithMetadata(md).True(condition.ReasonDetached))
	return nil
}

func (c *Controller) attachNetwork(ctx context.Context, task *tasksv1.Task, report *condition.Report) error {
	c.Report(report.Type(condition.NetworkReady).False(condition.ReasonAttaching))

	// id, err := c.runtime.ID(ctx, task.GetMeta().GetUid())
	// if err != nil {
	// 	_ = c.clientset.TaskV1().Condition(ctx, report.Type(condition.NetworkReady).False(condition.ReasonAttachFailed, err.Error()))
	// 	return err
	// }

	id := task.GetMeta().GetUid()
	c.logger.Debug("attaching mounts for task", "task", id)

	pid, err := c.runtime.Pid(ctx, id)
	if err != nil {
		if !errs.IsNotFound(err) {
			c.Report(report.Type(condition.NetworkReady).False(condition.ReasonAttachFailed, err.Error()))
		}
		return err
	}

	pm := networking.ParseCNIPortMappings(task.GetConfig().PortMappings...)
	attachOpts := []gocni.NamespaceOpts{gocni.WithCapabilityPortMap(pm), gocni.WithArgs("IgnoreUnknown", "true")}

	res, err := c.netmanager.Attach(ctx, id, pid, attachOpts...)
	if err != nil {
		if !errs.IsNotFound(err) {
			c.Report(report.Type(condition.NetworkReady).False(condition.ReasonAttachFailed, err.Error()))
		}
		return err
	}

	var ipaddr, gw string

	for i, inter := range res.Interfaces {
		for _, ipcfg := range inter.IPConfigs {
			if i == "eth1" {
				ipaddr = ipcfg.IP.String()
				gw = ipcfg.Gateway.String()
				break
			}
		}
	}

	md := map[string]string{
		"ip_address": ipaddr,
		"gateway":    gw,
	}

	c.Report(report.Type(condition.NetworkReady).WithMetadata(md).True(condition.ReasonAttached, ""))
	return nil
}
