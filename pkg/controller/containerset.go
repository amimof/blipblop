package controller

import (
	"context"
	"fmt"

	containersv1 "github.com/amimof/blipblop/api/services/containers/v1"
	containersetsv1 "github.com/amimof/blipblop/api/services/containersets/v1"
	eventsv1 "github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/api/types/v1"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/amimof/blipblop/pkg/events"
	"github.com/amimof/blipblop/pkg/labels"
	"github.com/amimof/blipblop/pkg/logger"
	"github.com/amimof/blipblop/pkg/util"
)

type ContainerSetController struct {
	clientset *client.ClientSet
	logger    logger.Logger
}

type NewContainerSetControllerOption func(c *ContainerSetController)

func WithContainerSetLogger(l logger.Logger) NewContainerSetControllerOption {
	return func(c *ContainerSetController) {
		c.logger = l
	}
}

func (c *ContainerSetController) Run(ctx context.Context) {
	// Subscribe to events
	_, err := c.clientset.EventV1().Subscribe(ctx, events.ALL...)

	// Setup Handlers
	c.clientset.EventV1().On(events.ContainerCreate, c.onCreate)
	c.clientset.EventV1().On(events.ContainerUpdate, func(ctx context.Context, e *eventsv1.Event) error {
		return nil
	})
	c.clientset.EventV1().On(events.ContainerDelete, c.onDelete)

	// Handle errors
	for e := range err {
		c.logger.Error("received error on channel", "error", e)
	}
}

func (c *ContainerSetController) onCreate(ctx context.Context, e *eventsv1.Event) error {
	var set containersetsv1.ContainerSet
	err := e.Object.UnmarshalTo(&set)
	if err != nil {
		return err
	}

	// Merge labels from containerset into container
	l := labels.New()
	l.Set(labels.LabelPrefix("container-set").String(), set.GetMeta().GetName())
	l.AppendMap(set.GetMeta().GetLabels())
	template := set.GetTemplate()

	// Create container instance
	container := &containersv1.Container{
		Meta: &types.Meta{
			Name:   fmt.Sprintf("%s-%s", set.GetMeta().GetName(), util.GenerateBase36(8)),
			Labels: l,
		},
		Config: template,
	}

	// Create container request
	err = c.clientset.ContainerV1().Create(ctx, container)
	if err != nil {
		return err
	}

	return nil
}

func (c *ContainerSetController) onDelete(ctx context.Context, e *eventsv1.Event) error {
	var containerSet containersetsv1.ContainerSet
	err := e.GetObject().UnmarshalTo(&containerSet)
	if err != nil {
		return err
	}

	ctrs, err := c.clientset.ContainerV1().List(ctx)
	if err != nil {
		return err
	}

	key := labels.LabelPrefix("container-set").String()

	for _, ctr := range ctrs {
		if _, ok := ctr.GetMeta().GetLabels()[key]; ok {
			if ctr.GetMeta().GetLabels()[key] == containerSet.GetMeta().GetName() {
				err = c.clientset.ContainerV1().Delete(ctx, ctr.GetMeta().GetName())
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func NewContainerSetController(cs *client.ClientSet, opts ...NewContainerSetControllerOption) *ContainerSetController {
	eh := &ContainerSetController{
		clientset: cs,
		logger:    logger.ConsoleLogger{},
	}

	for _, opt := range opts {
		opt(eh)
	}

	return eh
}
