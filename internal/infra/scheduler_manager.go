package infra

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/types/known/anypb"

	"github.com/amimof/voiyd/internal/app"
	"github.com/amimof/voiyd/pkg/events"
	"github.com/amimof/voiyd/pkg/logger"
	"github.com/amimof/voiyd/pkg/scheduling"

	eventsv1 "github.com/amimof/voiyd/api/services/events/v1"
	nodesv1 "github.com/amimof/voiyd/api/services/nodes/v1"
	tasksv1 "github.com/amimof/voiyd/api/services/tasks/v1"
)

type ScheduleManager struct {
	Scheduler   scheduling.Scheduler
	Logger      logger.Logger
	Exchange    *events.Exchange
	NodeService app.NodeSender
}

func (c *ScheduleManager) Kill(ctx context.Context, task *tasksv1.Task, nodeUID string) error {
	if !c.NodeService.IsNodeConnected(nodeUID) {
		c.Logger.Warn("target node not connected, cannot kill", "task", task.GetMeta().GetName(), "node", nodeUID)
		return fmt.Errorf("target node %s not connected", nodeUID)
	}

	// Create kill event
	event := events.NewEvent(events.TaskKill, task)

	// Send to target node
	if err := c.NodeService.SendToNode(ctx, nodeUID, event); err != nil {
		c.Logger.Error("failed to send kill event to node", "error", err, "task", task.GetMeta().GetName(), "node", nodeUID)
		return err
	}

	return nil
}

func (c *ScheduleManager) Stop(ctx context.Context, task *tasksv1.Task, nodeUID string) error {
	if !c.NodeService.IsNodeConnected(nodeUID) {
		c.Logger.Warn("target node not connected, cannot stop", "task", task.GetMeta().GetName(), "node", nodeUID)
		return fmt.Errorf("target node %s not connected", nodeUID)
	}

	// Create kill event
	event := events.NewEvent(events.TaskStop, task)

	// Send to target node
	if err := c.NodeService.SendToNode(ctx, nodeUID, event); err != nil {
		c.Logger.Error("failed to send stop event to node", "error", err, "task", task.GetMeta().GetName(), "node", nodeUID)
		return err
	}

	return nil
}

func (c *ScheduleManager) Start(ctx context.Context, task *tasksv1.Task) (*nodesv1.Node, error) {
	return c.Schedule(ctx, task)
}

// Schedule attempts to schedule a task to a suitable node.
func (c *ScheduleManager) Schedule(ctx context.Context, task *tasksv1.Task) (*nodesv1.Node, error) {
	taskID := task.GetMeta().GetUid()

	nodes, err := c.NodeService.List(ctx, 0)
	if err != nil {
		return nil, err
	}

	// Find a node fit for the task using a scheduler
	n, err := c.Scheduler.Schedule(ctx, task, nodes)
	if err != nil {
		c.Logger.Warn("error scheduling task", "task", taskID, "error", err)
		return nil, err
	}

	nodeUID := n.GetMeta().GetUid()

	// Check if target node is connected
	if !c.NodeService.IsNodeConnected(nodeUID) {
		c.Logger.Warn("target node not connected, cannot schedule", "task", task.GetMeta().GetName(), "node", n.GetMeta().GetName())
		return nil, fmt.Errorf("target node %s not connected", n.GetMeta().GetName())
	}

	taskpb, err := anypb.New(task)
	if err != nil {
		return nil, err
	}

	nodepb, err := anypb.New(n)
	if err != nil {
		return nil, err
	}

	// Create targeted schedule request
	scheduleReq := &eventsv1.ScheduleRequest{
		Task: taskpb,
		Node: nodepb,
	}

	// Create event
	event := events.NewEvent(events.Schedule, scheduleReq)
	// Send ONLY to target node
	if err := c.NodeService.SendToNode(ctx, nodeUID, event); err != nil {
		c.Logger.Error("failed to send schedule event to node", "error", err, "task", task.GetMeta().GetName(), "node", n.GetMeta().GetName())
		return nil, err
	}

	// Update task status AFTER successful send
	c.Logger.Info("scheduled task to node", "task", task.GetMeta().GetName(), "node", n.GetMeta().GetName())

	return n, nil
}
