package controller

import (
	"context"

	containersv1 "github.com/amimof/blipblop/api/services/containers/v1"
	eventsv1 "github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/amimof/blipblop/pkg/events"
	"github.com/amimof/blipblop/pkg/logger"
	"github.com/amimof/blipblop/pkg/scheduling"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type SchedulerController struct {
	clientset *client.ClientSet
	scheduler scheduling.Scheduler
	logger    logger.Logger
}

func (c *SchedulerController) handleErrors(h events.HandlerFunc) events.HandlerFunc {
	return func(ctx context.Context, ev *eventsv1.Event) error {
		err := h(ctx, ev)
		if err != nil {
			c.logger.Error("handler returned error", "event", ev.GetType().String(), "error", err)
			return err
		}
		return err
	}
}

func (c *SchedulerController) onContainerCreate(ctx context.Context, e *eventsv1.Event) error {
	// Get the container
	var ctr containersv1.Container
	err := e.Object.UnmarshalTo(&ctr)
	if err != nil {
		return err
	}

	// Find a node fit for the container using a scheduler
	n, err := c.scheduler.Schedule(ctx, &ctr)
	if err != nil {
		return err
	}

	// Update container status
	_ = c.clientset.ContainerV1().Status().Update(
		ctx,
		ctr.GetMeta().GetName(),
		&containersv1.Status{
			Phase: wrapperspb.String("scheduled"),
			Node:  wrapperspb.String(n.GetMeta().GetName()),
		},
		"phase")

	containerProto, err := anypb.New(&ctr)
	if err != nil {
		return err
	}

	nodeProto, err := anypb.New(n)
	if err != nil {
		return err
	}

	ev := &eventsv1.ScheduleRequest{Container: containerProto, Node: nodeProto}

	return c.clientset.EventV1().Publish(ctx, ev, eventsv1.EventType_Schedule)
}

func (c *SchedulerController) Run(ctx context.Context) {
	// Subscribe to events
	ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_controller_name", "scheduler")
	_, err := c.clientset.EventV1().Subscribe(ctx, events.ContainerCreate)

	// Setup Handlers
	c.clientset.EventV1().On(events.ContainerCreate, c.handleErrors(c.onContainerCreate))

	// Handle errors
	for e := range err {
		c.logger.Error("received error on channel", "error", e)
	}
}

func NewSchedulerController(cs *client.ClientSet, scheduler scheduling.Scheduler) *SchedulerController {
	return &SchedulerController{
		clientset: cs,
		scheduler: scheduler,
		logger:    logger.ConsoleLogger{},
	}
}
