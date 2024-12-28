package controller

import (
	"context"

	containersv1 "github.com/amimof/blipblop/api/services/containers/v1"
	eventsv1 "github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/amimof/blipblop/pkg/eventsv2"
	"github.com/amimof/blipblop/pkg/labels"
	"github.com/amimof/blipblop/pkg/logger"
	"github.com/amimof/blipblop/pkg/node"
	"github.com/amimof/blipblop/pkg/runtime"
)

type ContainerController struct {
	clientset *client.ClientSet
	runtime   runtime.Runtime
	logger    logger.Logger
}

type NewContainerControllerOption func(c *ContainerController)

func WithContainerControllerLogger(l logger.Logger) NewContainerControllerOption {
	return func(c *ContainerController) {
		c.logger = l
	}
}

func (c *ContainerController) Run(ctx context.Context, stopCh <-chan struct{}) {
	// Subscribe to events
	_, err := c.clientset.EventV1().Subscribe(ctx, eventsv2.ALL...)

	// Setup Handlers
	c.clientset.EventV1().On(eventsv2.Schedule, c.onContainerCreate)
	c.clientset.EventV1().On(eventsv2.ContainerUpdate, c.onContainerUpdate)
	c.clientset.EventV1().On(eventsv2.ContainerDelete, c.onContainerDelete)
	c.clientset.EventV1().On(eventsv2.ContainerStart, c.onContainerStart)
	c.clientset.EventV1().On(eventsv2.ContainerKill, c.onContainerKill)
	c.clientset.EventV1().On(eventsv2.ContainerStop, c.onContainerStop)

	// Handle errors
	for e := range err {
		c.logger.Error("received error on channel", "error", e)
	}
}

func (c *ContainerController) Reconcile(ctx context.Context) error {
	return nil
}

func (c *ContainerController) onContainerCreate(ctx context.Context, obj *eventsv1.Event) error {
	// Extract ScheduleRequest embedded in the event
	var req eventsv1.ScheduleRequest
	if err := obj.GetObject().UnmarshalTo(&req); err != nil {
		return err
	}

	// Get the container from the request
	var ctr containersv1.Container
	if err := req.GetContainer().UnmarshalTo(&ctr); err != nil {
		return err
	}

	// Get the node from the request
	// var node nodesv1.Node
	// if err := req.GetNode().UnmarshalTo(&node); err != nil {
	// 	return err
	// }
	// Get the container
	// var ctr containersv1.Container
	// err := obj.Object.UnmarshalTo(&ctr)
	// if err != nil {
	// 	return err
	// }

	// Get the node from node config
	n, err := node.LoadNodeFromEnv("/etc/blipblop/node.yaml")
	if err != nil {
		return err
	}

	// Retreive latest node from the server
	n, err = c.clientset.NodeV1().Get(ctx, n.GetMeta().GetName())
	if err != nil {
		return err
	}

	// See if nodeSelector matches labels on the node
	nodeSelector := labels.NewCompositeSelectorFromMap(ctr.GetConfig().GetNodeSelector())
	if !nodeSelector.Matches(n.GetMeta().GetLabels()) {
		return nil
	}

	err = c.runtime.Pull(ctx, &ctr)
	if err != nil {
		return err
	}
	err = c.runtime.Run(ctx, &ctr)
	if err != nil {
		return err
	}
	return nil
}

func (c *ContainerController) onContainerUpdate(ctx context.Context, _ *eventsv1.Event) error {
	return nil
}

func (c *ContainerController) onContainerDelete(ctx context.Context, obj *eventsv1.Event) error {
	var ctr containersv1.Container
	err := obj.Object.UnmarshalTo(&ctr)
	if err != nil {
		return err
	}
	err = c.runtime.Kill(ctx, &ctr)
	if err != nil {
		return err
	}
	err = c.runtime.Delete(ctx, &ctr)
	if err != nil {
		return err
	}
	return nil
}

func (c *ContainerController) onContainerStart(ctx context.Context, obj *eventsv1.Event) error {
	var ctr containersv1.Container
	err := obj.Object.UnmarshalTo(&ctr)
	if err != nil {
		return err
	}

	// Delete container if it exists
	_ = c.clientset.ContainerV1().SetTaskStatus(ctx, ctr.GetMeta().GetName(), containersv1.Phase_Stopping.String(), "")
	if err := c.runtime.Delete(ctx, &ctr); err != nil {
		return err
	}

	// Pull image
	_ = c.clientset.ContainerV1().SetTaskStatus(ctx, ctr.GetMeta().GetName(), containersv1.Phase_Pulling.String(), "")
	err = c.runtime.Pull(ctx, &ctr)
	if err != nil {
		return err
	}

	// Run container
	_ = c.clientset.ContainerV1().SetTaskStatus(ctx, ctr.GetMeta().GetName(), containersv1.Phase_Starting.String(), "")
	err = c.runtime.Run(ctx, &ctr)
	if err != nil {
		return err
	}

	return nil
}

func (c *ContainerController) onContainerKill(ctx context.Context, obj *eventsv1.Event) error {
	var ctr containersv1.Container
	err := obj.Object.UnmarshalTo(&ctr)
	if err != nil {
		return err
	}

	err = c.runtime.Kill(ctx, &ctr)
	if err != nil {
		return err
	}
	return nil
}

func (c *ContainerController) onContainerStop(ctx context.Context, obj *eventsv1.Event) error {
	var ctr containersv1.Container
	err := obj.Object.UnmarshalTo(&ctr)
	if err != nil {
		return err
	}

	err = c.runtime.Stop(ctx, &ctr)
	if err != nil {
		return err
	}
	return nil
}

func NewContainerController(cs *client.ClientSet, rt runtime.Runtime, opts ...NewContainerControllerOption) *ContainerController {
	eh := &ContainerController{
		clientset: cs,
		runtime:   rt,
		logger:    logger.ConsoleLogger{},
	}

	for _, opt := range opts {
		opt(eh)
	}

	return eh
}
