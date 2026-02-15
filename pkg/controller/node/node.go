// Package nodecontroller implemenets controller and provides logic for multiplexing node management
package nodecontroller

import (
	"context"
	"os"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/metadata"

	"github.com/amimof/voiyd/pkg/client"
	"github.com/amimof/voiyd/pkg/condition"
	"github.com/amimof/voiyd/pkg/events"
	"github.com/amimof/voiyd/pkg/logger"
	"github.com/amimof/voiyd/pkg/networking"
	"github.com/amimof/voiyd/pkg/queue"
	"github.com/amimof/voiyd/pkg/runtime"
	"github.com/amimof/voiyd/pkg/store"
	"github.com/amimof/voiyd/pkg/volume"

	eventsv1 "github.com/amimof/voiyd/api/services/events/v1"
	logsv1 "github.com/amimof/voiyd/api/services/logs/v1"
	nodesv1 "github.com/amimof/voiyd/api/services/nodes/v1"
)

type Controller struct {
	runtime          runtime.Runtime
	logger           logger.Logger
	clientset        *client.ClientSet
	tracer           trace.Tracer
	logChan          chan *logsv1.LogEntry
	activeLogStreams map[events.LogKey]context.CancelFunc
	logStreamsMu     sync.RWMutex
	node             *nodesv1.Node
	attacher         volume.Attacher
	exchange         *events.Exchange
	renewInterval    time.Duration
	netmanager       networking.Manager
	tokenStore       store.Store
	leaseTokens      map[string]string

	mu       sync.Mutex
	queue    *queue.TaskQueue
	workPool *queue.WorkPool
}

type NewOption func(c *Controller)

func WithLeaseRenewalInterval(d time.Duration) NewOption {
	return func(c *Controller) {
		c.renewInterval = d
	}
}

func WithVolumeAttacher(a volume.Attacher) NewOption {
	return func(c *Controller) {
		c.attacher = a
	}
}

func WithConfig(n *nodesv1.Node) NewOption {
	return func(c *Controller) {
		c.node = n
	}
}

func WithLogger(l logger.Logger) NewOption {
	return func(c *Controller) {
		c.logger = l
	}
}

func WithName(s string) NewOption {
	return func(c *Controller) {
		c.node.GetMeta().Name = s
	}
}

func WithExchange(e *events.Exchange) NewOption {
	return func(c *Controller) {
		c.exchange = e
	}
}

func WithNetworkManager(m networking.Manager) NewOption {
	return func(c *Controller) {
		c.netmanager = m
	}
}

func WithTokenStore(s store.Store) NewOption {
	return func(c *Controller) {
		c.tokenStore = s
	}
}

// Run implements controller
func (c *Controller) Run(ctx context.Context) {
	node, err := c.getNode(ctx)
	if err != nil {
		c.logger.Error("error getting node", "error", err)
	}
	nodeUID := node.GetMeta().GetUid()
	nodeName := c.node.GetMeta().GetName()

	// init work pool
	c.workPool.Start(ctx)
	defer c.workPool.Stop()

	topics := []eventsv1.EventType{
		events.TailLogsStart,
		events.TailLogsStop,
	}

	// Subscribe to events
	ctx = metadata.AppendToOutgoingContext(ctx, "voiyd_controller_name", "node")
	evt, errCh := c.clientset.EventV1().Subscribe(ctx, topics...)

	c.clientset.EventV1().On(events.TailLogsStart, events.HandleErrors(c.logger, c.onLogStart))
	c.clientset.EventV1().On(events.TailLogsStop, events.HandleErrors(c.logger, c.onLogStop))

	go func() {
		for e := range evt {
			c.logger.Info("node controller received event", "event", e.GetType().String(), "clientID", nodeName, "objectID", e.GetObjectId())
		}
	}()

	// Setup Node Handlers
	c.exchange.On(events.Schedule, events.HandleErrors(c.logger, events.HandleScheduling(c.processScheduleTask)))
	c.exchange.On(events.TaskDelete, events.HandleErrors(c.logger, events.HandleTask(c.processStopTask)))
	c.exchange.On(events.TaskStop, events.HandleErrors(c.logger, events.HandleTask(c.processStopTask)))
	c.exchange.On(events.TaskKill, events.HandleErrors(c.logger, events.HandleTask(c.processKillTask)))

	// Handle runtime events
	runtimeChan := c.exchange.Subscribe(ctx, events.RuntimeTaskExit, events.RuntimeTaskStart, events.RuntimeTaskDelete)
	c.exchange.On(events.RuntimeTaskExit, events.HandleErrors(c.logger, c.onRuntimeTaskExit))
	c.exchange.On(events.RuntimeTaskStart, events.HandleErrors(c.logger, c.onRuntimeTaskStart))
	c.exchange.On(events.RuntimeTaskDelete, events.HandleErrors(c.logger, c.onRuntimeTaskDelete))

	go func() {
		for e := range runtimeChan {
			c.logger.Info("node controller received runtime event", "event", e.GetType().String(), "objectID", e.GetObjectId())
		}
	}()

	// Connect with retry logic
	connErr := make(chan error, 1)
	connEvt := make(chan *eventsv1.Event)
	go func() {
		err := c.clientset.NodeV1().Connect(ctx, nodeUID, nodeName, connEvt, connErr)
		if err != nil {
			c.logger.Error("error connecting to server", "error", err)
		}
	}()
	go func() {
		for e := range connEvt {
			c.logger.Info("node controller received targeted event", "event", e.GetType().String(), "clientID", nodeName, "objectID", e.GetObjectId())

			// CRITICAL: Publish to local exchange so handlers can process it!
			if err := c.exchange.Publish(ctx, e); err != nil {
				c.logger.Error("failed to publish event to local exchange", "error", err, "eventType", e.GetType().String())
			}
		}
	}()

	// Reconcile
	go func() {
		if err := c.Reconcile(ctx); err != nil {
			c.logger.Warn("error reconciling", "error", err, "node", nodeName)
		}
	}()

	// Get hostname from environment
	hostname, err := os.Hostname()
	if err != nil {
		c.logger.Error("error retrieving hostname from environment", "error", err)
	}

	// Get version from runtime
	runtimeVer, err := c.runtime.Version(ctx)
	if err != nil {
		c.logger.Error("error retrieving version from runtime", "error", err)
	}

	// Report node status with metadata
	if node, err := c.clientset.NodeV1().Get(ctx, nodeName); err == nil {
		reporter := condition.NewForResource(node)
		_ = c.clientset.NodeV1().Condition(ctx, reporter.Type(condition.NodeReady).WithMetadata(map[string]string{"hostname": hostname, "runtime_version": runtimeVer}).True(condition.ReasonConnected))
	}

	// Start lease loop
	go c.renewLeases(ctx)

	// Handle errors
	for {
		select {
		case <-ctx.Done():
			return
		case e, ok := <-errCh:
			if !ok {
				errCh = nil
				continue
			}
			if e != nil {
				c.logger.Error("received error on channel", "error", e)
			}
		case e, ok := <-connErr:
			if !ok {
				connErr = nil
				continue
			}
			if e != nil {
				c.logger.Error("received error on channel", "error", e)
			}
		}
	}
}

// renewLeases continuously renews leases for all running tasks
func (c *Controller) renewLeases(ctx context.Context) {
	ticker := time.NewTicker(c.renewInterval)
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

func (c *Controller) renewAllLeases(ctx context.Context) {
	// Get all running tasks on this node from runtime
	tasks, err := c.runtime.List(ctx)
	if err != nil {
		c.logger.Error("failed to list runtime tasks", "error", err)
		return
	}

	// Loop though runtime tasks and try to find an existing refresh token for it.
	// - Task found in runtime but no refresh token in store = stop and remove task
	// - Task found in both runtime and store = try to renew
	// - If renew fails, stop and remove the task since its probably held by someone else
	for _, task := range tasks {

		taskName := task.GetMeta().GetName()
		taskUID := task.GetMeta().GetUid()

		refreshToken, err := c.tokenStore.Load(taskUID)
		if err != nil {
			c.logger.Debug("error loading refresh token from store", "error", err, "task", taskName)
			_ = c.stopTask(ctx, task)
			continue
		}

		err = c.renewLease(ctx, task, string(refreshToken))
		if err != nil {
			c.logger.Debug("error renewing lease", "error", err, "task", taskName)
			_ = c.stopTask(ctx, task)
			continue
		}

	}
}

// Reconcile ensures that desired tasks matches with tasks
// in the runtime environment. It removes any tasks that are not
// desired (missing from the server) and adds those missing from runtime.
// It is preferrably run early during startup of the controller.
func (c *Controller) Reconcile(ctx context.Context) error {
	c.renewAllLeases(ctx)
	return nil
}

func New(c *client.ClientSet, n *nodesv1.Node, rt runtime.Runtime, opts ...NewOption) (*Controller, error) {
	m := &Controller{
		clientset:        c,
		runtime:          rt,
		netmanager:       &networking.UnimplementedManager{},
		logger:           logger.ConsoleLogger{},
		tracer:           otel.Tracer("controller"),
		logChan:          make(chan *logsv1.LogEntry),
		activeLogStreams: make(map[events.LogKey]context.CancelFunc),
		node:             n,
		attacher:         volume.NewDefaultAttacher(c.VolumeV1()),
		renewInterval:    time.Second * 30,
		leaseTokens:      make(map[string]string),
	}

	for _, opt := range opts {
		opt(m)
	}

	// NEW: Initialize queue and workpool
	m.queue = queue.NewTaskQueue(m.logger)
	m.workPool = queue.NewPool(
		m.queue,
		queue.WithMaxWorkers(2),
		queue.WithMaxRetries(5),
		queue.WithBackoff(5*time.Second, 15*time.Second),
		queue.WithLogger(m.logger),
	)
	return m, nil
}
