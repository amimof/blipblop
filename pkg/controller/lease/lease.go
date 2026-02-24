package leasecontroller

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"github.com/amimof/voiyd/pkg/client"
	"github.com/amimof/voiyd/pkg/errs"
	"github.com/amimof/voiyd/pkg/events"
	"github.com/amimof/voiyd/pkg/logger"
)

type Controller struct {
	logger    logger.Logger
	clientset *client.ClientSet
	tracer    trace.Tracer
	exchange  *events.Exchange
}

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

func (c *Controller) renewAllLeases(ctx context.Context) {
	leases, err := c.clientset.LeaseV1().List(ctx)
	if err != nil {
		c.logger.Error("error listing leases", "error", err)
		return
	}

	for _, lease := range leases {
		// Checks if task exists
		_, err := c.clientset.TaskV1().Get(ctx, lease.GetConfig().GetTaskId())
		if errs.IsNotFound(err) {
			if err := c.clientset.LeaseV1().Revoke(ctx, lease.GetConfig().GetTaskId(), lease.GetConfig().GetNodeId()); err != nil {
				c.logger.Error("error revoking orphaned lease", "error", err, "task", lease.GetConfig().GetTaskId())
				continue
			}
		}
		// Checks if lease has expired
		if time.Now().After(lease.GetConfig().GetExpiresAt().AsTime()) {
			err = c.exchange.Forward(ctx, events.NewEvent(events.LeaseExpired, lease))
			if err != nil {
				c.logger.Error("error forwarding LeaseExpired event", "error", err, "task", lease.GetConfig().GetTaskId())
				continue
			}
		}
	}
}

func (c *Controller) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second * 3)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.renewAllLeases(ctx)
		}
	}
}

func New(cs *client.ClientSet, opts ...NewOption) *Controller {
	m := &Controller{
		clientset: cs,
		logger:    logger.ConsoleLogger{},
		tracer:    otel.Tracer("controller"),
	}
	for _, opt := range opts {
		opt(m)
	}

	return m
}
