package containersetcontroller

import (
	"context"
	"fmt"

	"google.golang.org/grpc/metadata"

	"github.com/amimof/voiyd/api/types/v1"
	"github.com/amimof/voiyd/pkg/client"
	"github.com/amimof/voiyd/pkg/events"
	"github.com/amimof/voiyd/pkg/labels"
	"github.com/amimof/voiyd/pkg/logger"
	"github.com/amimof/voiyd/pkg/util"

	containersetsv1 "github.com/amimof/voiyd/api/services/containersets/v1"
	eventsv1 "github.com/amimof/voiyd/api/services/events/v1"
	tasksv1 "github.com/amimof/voiyd/api/services/tasks/v1"
)

type Controller struct {
	clientset *client.ClientSet
	logger    logger.Logger
}

type NewOption func(c *Controller)

func WithLogger(l logger.Logger) NewOption {
	return func(c *Controller) {
		c.logger = l
	}
}

func (c *Controller) Run(ctx context.Context) {
	// Subscribe to events
	ctx = metadata.AppendToOutgoingContext(ctx, "voiyd_controller_name", "continersetcontroller")
	_, err := c.clientset.EventV1().Subscribe(ctx, events.ALL...)

	// Setup Handlers
	c.clientset.EventV1().On(events.TaskCreate, c.onCreate)
	c.clientset.EventV1().On(events.TaskUpdate, func(ctx context.Context, e *eventsv1.Event) error {
		return nil
	})
	c.clientset.EventV1().On(events.TaskDelete, c.onDelete)

	// Handle errors
	for e := range err {
		c.logger.Error("received error on channel", "error", e)
	}
}

func (c *Controller) onCreate(ctx context.Context, e *eventsv1.Event) error {
	var set containersetsv1.ContainerSet
	err := e.Object.UnmarshalTo(&set)
	if err != nil {
		return err
	}

	// Merge labels from containerset into task
	l := labels.New()
	l.Set(labels.LabelPrefix("task-set").String(), set.GetMeta().GetName())
	l.AppendMap(set.GetMeta().GetLabels())
	template := set.GetTemplate()

	// Create task instance
	task := &tasksv1.Task{
		Meta: &types.Meta{
			Name:   fmt.Sprintf("%s-%s", set.GetMeta().GetName(), util.GenerateBase36(8)),
			Labels: l,
		},
		Config: template,
	}

	// Create task request
	err = c.clientset.TaskV1().Create(ctx, task)
	if err != nil {
		return err
	}

	return nil
}

func (c *Controller) onDelete(ctx context.Context, e *eventsv1.Event) error {
	var taskSet containersetsv1.ContainerSet
	err := e.GetObject().UnmarshalTo(&taskSet)
	if err != nil {
		return err
	}

	ctrs, err := c.clientset.TaskV1().List(ctx)
	if err != nil {
		return err
	}

	key := labels.LabelPrefix("task-set").String()

	for _, ctr := range ctrs {
		if _, ok := ctr.GetMeta().GetLabels()[key]; ok {
			if ctr.GetMeta().GetLabels()[key] == taskSet.GetMeta().GetName() {
				err = c.clientset.TaskV1().Delete(ctx, ctr.GetMeta().GetName())
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func New(cs *client.ClientSet, opts ...NewOption) *Controller {
	eh := &Controller{
		clientset: cs,
		logger:    logger.ConsoleLogger{},
	}

	for _, opt := range opts {
		opt(eh)
	}

	return eh
}
