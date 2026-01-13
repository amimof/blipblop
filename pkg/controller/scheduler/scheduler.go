package schedulercontroller

import (
	"context"

	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/amimof/voiyd/pkg/client"
	"github.com/amimof/voiyd/pkg/events"
	"github.com/amimof/voiyd/pkg/logger"
	"github.com/amimof/voiyd/pkg/scheduling"

	eventsv1 "github.com/amimof/voiyd/api/services/events/v1"
	tasksv1 "github.com/amimof/voiyd/api/services/tasks/v1"
)

type NewOption func(c *Controller)

func WithLogger(l logger.Logger) NewOption {
	return func(c *Controller) {
		c.logger = l
	}
}

func WithExchange(e *events.Exchange) NewOption {
	return func(c *Controller) {
		c.exchange = e
	}
}

type Controller struct {
	clientset *client.ClientSet
	scheduler scheduling.Scheduler
	logger    logger.Logger
	exchange  *events.Exchange
}

func (c *Controller) handleErrors(h events.HandlerFunc) events.HandlerFunc {
	return func(ctx context.Context, ev *eventsv1.Event) error {
		err := h(ctx, ev)
		if err != nil {
			c.logger.Error("handler returned error", "event", ev.GetType().String(), "error", err)
			return err
		}
		return err
	}
}

func (c *Controller) onTaskCreate(ctx context.Context, e *eventsv1.Event) error {
	// Get the container
	var ctr tasksv1.Task
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
	_ = c.clientset.TaskV1().Status().Update(
		ctx,
		ctr.GetMeta().GetName(),
		&tasksv1.Status{
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

	ev := &eventsv1.ScheduleRequest{Task: containerProto, Node: nodeProto}

	// return c.clientset.EventV1().Publish(ctx, ev, eventsv1.EventType_Schedule)
	err = c.exchange.Publish(ctx, events.NewEvent(events.Schedule, ev))
	if err != nil {
		return err
	}
	// return c.exchange.Publish(ctx, ev, eventsv1.EventType_Schedule)
	return nil
}

func (c *Controller) onNodeForget(ctx context.Context, e *eventsv1.Event) error {
	// var node nodesv1.Node
	// err := e.GetObject().UnmarshalTo(&node)
	// if err != nil {
	// 	return err
	// }
	//
	// // Re-emit start-task events for tasks that the node had
	// tasks, err := c.clientset.TaskV1().List(ctx)
	// if err != nil {
	// 	c.logger.Error("error listing tasks", "error", err)
	// 	return err
	// }
	//
	// for _, task := range tasks {
	// 	if task.GetStatus().GetNode().GetValue() == node.GetMeta().GetName() {
	// 		err = c.clientset.EventV1().Publish(ctx, task, events.TaskStart)
	// 		if err != nil {
	// 			c.logger.Error("error emitting start event", "error", err, "task", task.GetMeta().GetName())
	// 			continue
	// 		}
	// 	}
	// }
	//
	// c.logger.Debug("forgetting node", "node", node.GetMeta().GetName())
	return nil
}

func (c *Controller) Run(ctx context.Context) {
	// Subscribe to events
	ctx = metadata.AppendToOutgoingContext(ctx, "voiyd_controller_name", "scheduler")
	_, err := c.clientset.EventV1().Subscribe(ctx, events.TaskCreate, events.NodeForget)

	// Setup Handlers
	c.clientset.EventV1().On(events.TaskCreate, c.handleErrors(c.onTaskCreate))
	c.clientset.EventV1().On(events.NodeForget, c.handleErrors(c.onNodeForget))

	// Handle errors
	for e := range err {
		c.logger.Error("received error on channel", "error", e)
	}
}

func New(cs *client.ClientSet, scheduler scheduling.Scheduler, opts ...NewOption) *Controller {
	c := &Controller{
		clientset: cs,
		scheduler: scheduler,
		logger:    logger.ConsoleLogger{},
	}
	for _, opt := range opts {
		opt(c)
	}

	return c
}
