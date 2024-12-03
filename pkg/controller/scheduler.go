package controller

import (
	"context"
	"time"

	containersv1 "github.com/amimof/blipblop/api/services/containers/v1"
	eventsv1 "github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/amimof/blipblop/pkg/events/informer"
	"github.com/amimof/blipblop/pkg/logger"
	"github.com/amimof/blipblop/pkg/scheduling"
	"google.golang.org/protobuf/types/known/anypb"
)

type SchedulerController struct {
	clientset *client.ClientSet
	scheduler scheduling.Scheduler
	logger    logger.Logger
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
	_ = c.clientset.ContainerV1().SetTaskStatus(ctx, ctr.GetMeta().GetName(), containersv1.Phase_Scheduling.String(), "")

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
	// Setup channels
	evt := make(chan *eventsv1.Event, 10)
	errChan := make(chan error, 10)

	// Setup handlers
	handlers := informer.ContainerEventHandlerFuncs{
		OnCreate: c.onContainerCreate,
		// OnUpdate: c.onContainerUpdate,
		// OnDelete: c.onContainerDelete,
		// OnStart:  c.onContainerStart,
		// OnKill:   c.onContainerKill,
		// OnStop:   c.onContainerStop,
	}

	// Run ctrInformer
	ctrInformer := informer.NewContainerEventInformer(handlers)
	go ctrInformer.Run(ctx, evt)

	// Subscribe with retry
	for {
		select {
		case <-ctx.Done():
			c.logger.Info("done watching, stopping subscription")
			return
		default:
			if err := c.clientset.EventV1().Subscribe(ctx, evt, errChan); err != nil {
				c.logger.Error("error occured during subscribe", "error", err)
			}
			c.logger.Info("attempting to re-subscribe to event server")
			time.Sleep(5 * time.Second)
		}
	}
}

func NewSchedulerController(cs *client.ClientSet, scheduler scheduling.Scheduler) *SchedulerController {
	return &SchedulerController{
		clientset: cs,
		scheduler: scheduler,
		logger:    logger.ConsoleLogger{},
	}
}
