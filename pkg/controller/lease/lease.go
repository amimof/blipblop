package leasecontroller

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	leasesv1 "github.com/amimof/voiyd/api/services/leases/v1"
	tasksv1 "github.com/amimof/voiyd/api/services/tasks/v1"
	"github.com/amimof/voiyd/pkg/client"
	"github.com/amimof/voiyd/pkg/events"
	"github.com/amimof/voiyd/pkg/logger"
)

type Controller struct {
	logger    logger.Logger
	clientset *client.ClientSet
	tracer    trace.Tracer
	exchange  *events.Exchange

	mu     sync.Mutex
	timers map[string]*time.Timer // keyed by task ID
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

// trackLease sets (or resets) a timer for the given task ID that fires at expiresAt.
// When the timer fires, it re-reads the lease from the store. If the lease was renewed
// (ExpiresAt is now in the future), a new timer is set. Otherwise, the lease is
// considered truly expired and a LeaseExpired event is forwarded.
func (c *Controller) trackLease(ctx context.Context, taskID string, expiresAt time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Stop any existing timer for this task
	if t, ok := c.timers[taskID]; ok {
		t.Stop()
	}

	d := time.Until(expiresAt)
	if d < 0 {
		d = 0
	}

	c.timers[taskID] = time.AfterFunc(d, func() {
		c.onTimerFired(ctx, taskID)
	})
}

// stopTracking stops and removes the timer for the given task ID.
func (c *Controller) stopTracking(taskID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if t, ok := c.timers[taskID]; ok {
		t.Stop()
		delete(c.timers, taskID)
	}
}

// stopAllTimers stops all active timers. Called on shutdown.
func (c *Controller) stopAllTimers() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for id, t := range c.timers {
		t.Stop()
		delete(c.timers, id)
	}
}

// onTimerFired is called when a lease timer expires. It performs the "lazy check"
// by re-reading the lease from the store.
func (c *Controller) onTimerFired(ctx context.Context, taskID string) {
	lease, err := c.clientset.LeaseV1().Get(ctx, taskID)
	if err != nil {
		// Lease no longer exists — clean up tracking
		c.logger.Debug("lease not found on timer fire, stopping tracking", "task", taskID)
		c.stopTracking(taskID)
		return
	}

	expiresAt := lease.GetConfig().GetExpiresAt().AsTime()
	now := time.Now()

	if now.Before(expiresAt) {
		// Lease was renewed since we set the timer — reschedule
		c.logger.Debug("lease was renewed, rescheduling timer", "task", taskID, "expiresAt", expiresAt)
		c.trackLease(ctx, taskID, expiresAt)
		return
	}

	// Lease is truly expired — forward the event
	c.logger.Info("lease expired", "task", taskID, "node", lease.GetConfig().GetNodeId())
	if err := c.exchange.Forward(ctx, events.NewEvent(events.LeaseExpired, lease)); err != nil {
		c.logger.Error("error forwarding LeaseExpired event", "error", err, "task", taskID)
	}

	// Clean up tracking for this expired lease
	c.stopTracking(taskID)
}

// onLeaseAcquired handles LeaseAcquired events by starting to track the new lease.
func (c *Controller) onLeaseAcquired(ctx context.Context, lease *leasesv1.Lease) error {
	taskID := lease.GetConfig().GetTaskId()
	expiresAt := lease.GetConfig().GetExpiresAt().AsTime()
	c.logger.Debug("tracking new lease", "task", taskID, "expiresAt", expiresAt)
	c.trackLease(ctx, taskID, expiresAt)
	return nil
}

// onTaskDeleted handles TaskDelete events by revoking the lease and stopping tracking.
func (c *Controller) onTaskDeleted(ctx context.Context, task *tasksv1.Task) error {
	taskID := task.GetMeta().GetName()
	c.logger.Debug("task deleted, revoking lease and stopping tracking", "task", taskID)

	// Stop tracking first
	c.stopTracking(taskID)

	// Try to revoke the lease — look up the lease to get the node ID
	lease, err := c.clientset.LeaseV1().Get(ctx, taskID)
	if err != nil {
		// Lease may not exist, that's fine
		c.logger.Debug("no lease found for deleted task", "task", taskID)
		return nil
	}

	if err := c.clientset.LeaseV1().Revoke(ctx, taskID, lease.GetConfig().GetNodeId()); err != nil {
		c.logger.Error("error revoking lease for deleted task", "error", err, "task", taskID)
		return err
	}

	return nil
}

// recoverTimers lists all existing leases and creates timers for each.
// Already-expired leases will fire immediately (duration <= 0).
func (c *Controller) recoverTimers(ctx context.Context) {
	leases, err := c.clientset.LeaseV1().List(ctx)
	if err != nil {
		c.logger.Error("error listing leases during recovery", "error", err)
		return
	}

	for _, lease := range leases {
		taskID := lease.GetConfig().GetTaskId()
		expiresAt := lease.GetConfig().GetExpiresAt().AsTime()
		c.logger.Debug("recovering timer for lease", "task", taskID, "expiresAt", expiresAt)
		c.trackLease(ctx, taskID, expiresAt)
	}

	c.logger.Info("recovered lease timers", "count", len(leases))
}

func (c *Controller) Run(ctx context.Context) {
	// Subscribe to events via the exchange
	c.exchange.On(events.LeaseAcquiered, events.HandleErrors(c.logger, events.HandleLease(c.onLeaseAcquired)))
	c.exchange.On(events.TaskDelete, events.HandleErrors(c.logger, events.HandleTask(c.onTaskDeleted)))

	// Recover timers for existing leases
	c.recoverTimers(ctx)

	// Block until context is cancelled
	<-ctx.Done()

	// Clean up all timers on shutdown
	c.stopAllTimers()
}

func New(cs *client.ClientSet, opts ...NewOption) *Controller {
	m := &Controller{
		clientset: cs,
		logger:    logger.ConsoleLogger{},
		tracer:    otel.Tracer("controller"),
		timers:    make(map[string]*time.Timer),
	}
	for _, opt := range opts {
		opt(m)
	}

	return m
}
