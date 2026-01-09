package containerdcontroller

import (
	"context"
	"fmt"
	"time"

	"github.com/containerd/containerd/api/events"
	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	typeurl "github.com/containerd/typeurl/v2"

	voiyd_events "github.com/amimof/voiyd/pkg/events"
	"github.com/amimof/voiyd/pkg/logger"
	"github.com/amimof/voiyd/pkg/runtime"
)

type Controller struct {
	client   *containerd.Client
	handlers *RuntimeHandlerFuncs
	runtime  runtime.Runtime
	logger   logger.Logger
	exchange *voiyd_events.Exchange
}

type NewOption func(*Controller)

func WithExchange(e *voiyd_events.Exchange) NewOption {
	return func(c *Controller) {
		c.exchange = e
	}
}

func WithLogger(l logger.Logger) NewOption {
	return func(c *Controller) {
		c.logger = l
	}
}

type Event interface {
	ProtoMessage()
}

type RuntimeHandlerFuncs struct {
	OnTaskExit         func(*events.TaskExit)
	OnTaskCreate       func(*events.TaskCreate)
	OnTaskStart        func(*events.TaskStart)
	OnTaskDelete       func(*events.TaskDelete)
	OnTaskIO           func(*events.TaskIO)
	OnTaskOOM          func(*events.TaskOOM)
	OnTaskExecAdded    func(*events.TaskExecAdded)
	OnTaskExecStarted  func(*events.TaskExecStarted)
	OnTaskPaused       func(*events.TaskPaused)
	OnTaskResumed      func(*events.TaskResumed)
	OnTaskCheckpointed func(*events.TaskCheckpointed)
	OnSnapshotPrepare  func(*events.SnapshotPrepare)
	OnSnapshotCommit   func(*events.SnapshotCommit)
	OnSnapshotRemove   func(*events.SnapshotRemove)
	OnNamespaceCreate  func(*events.NamespaceCreate)
	OnNamespaceUpdate  func(*events.NamespaceUpdate)
	OnNamespaceDelete  func(*events.NamespaceDelete)
	OnImageCreate      func(*events.ImageCreate)
	OnImageUpdate      func(*events.ImageUpdate)
	OnImageDelete      func(*events.ImageDelete)
	OnContainerCreate  func(*events.ContainerCreate)
	OnContainerUpdate  func(*events.ContainerUpdate)
	OnContainerDelete  func(*events.ContainerDelete)
	OnContentCreate    func(*events.ContentCreate)
	OnContentDelete    func(*events.ContentDelete)
}

func (c *Controller) AddHandler(h *RuntimeHandlerFuncs) {
	c.handlers = h
}

func connectTaskd(address string) (*containerd.Client, error) {
	client, err := containerd.New(address)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to containerd: %w", err)
	}
	return client, nil
}

func reconnectWithBackoff(address string, l logger.Logger) (*containerd.Client, error) {
	var (
		client *containerd.Client
		err    error
	)

	// Exponential backoff
	// TODO: Parameterize the backoff so it's not hardcoded
	backoff := 2 * time.Second
	for {
		client, err = connectTaskd(address)
		if err == nil {
			l.Info("successfully connected to containerd", "address", address)
			return client, nil
		}

		l.Error("error reconnecting to containerd", "error", err, "retry_in", backoff)
		time.Sleep(backoff)
	}
}

func (c *Controller) Run(ctx context.Context) {
	ctx = namespaces.WithNamespace(ctx, c.runtime.Namespace())

	err := c.Reconcile(ctx)
	if err != nil {
		c.logger.Error("error reconciling state", "error", err)
		return
	}
	err = c.streamEvents(ctx)
	if err != nil {
		c.logger.Info("Reconnecting stream")
		c.client, err = reconnectWithBackoff("/run/containerd/containerd.sock", c.logger)
		if err != nil {
			c.logger.Error("error reconnection to stream", "error", err)
		}
	}
}

func (c *Controller) streamEvents(ctx context.Context) error {
	filters := []string{fmt.Sprintf("namespace==%s", c.runtime.Namespace())}
	eventCh, errCh := c.client.Subscribe(ctx, filters...)
	for {
		select {
		case event := <-eventCh:
			ev, err := typeurl.UnmarshalAny(event.Event)
			if err != nil {
				c.logger.Error("error unmarshaling event received from stream", "error", err)
			}
			c.HandleEvent(c.handlers, ev)
		case err := <-errCh:
			return err
		case <-ctx.Done():
			if err := c.client.Close(); err != nil {
				c.logger.Error("error closing runtime client connection", "error", err)
			}
		}
	}
}

func (c *Controller) HandleEvent(handlers *RuntimeHandlerFuncs, obj any) {
	switch t := obj.(type) {
	case *events.TaskExit:
		if handlers.OnTaskExit != nil {
			handlers.OnTaskExit(t)
		}
	case *events.TaskCreate:
		if handlers.OnTaskCreate != nil {
			handlers.OnTaskCreate(t)
		}
	case *events.TaskStart:
		if handlers.OnTaskStart != nil {
			handlers.OnTaskStart(t)
		}
	case *events.TaskDelete:
		if handlers.OnTaskDelete != nil {
			handlers.OnTaskDelete(t)
		}
	case *events.TaskIO:
		if handlers.OnTaskIO != nil {
			handlers.OnTaskIO(t)
		}
	case *events.TaskOOM:
		if handlers.OnTaskOOM != nil {
			handlers.OnTaskOOM(t)
		}
	case *events.TaskExecAdded:
		if handlers.OnTaskExecAdded != nil {
			handlers.OnTaskExecAdded(t)
		}
	case *events.TaskExecStarted:
		if handlers.OnTaskExecStarted != nil {
			handlers.OnTaskExecStarted(t)
		}
	case *events.TaskPaused:
		if handlers.OnTaskPaused != nil {
			handlers.OnTaskPaused(t)
		}
	case *events.TaskResumed:
		if handlers.OnTaskResumed != nil {
			handlers.OnTaskResumed(t)
		}
	case *events.TaskCheckpointed:
		if handlers.OnTaskCheckpointed != nil {
			handlers.OnTaskCheckpointed(t)
		}
	case *events.SnapshotPrepare:
		if handlers.OnSnapshotPrepare != nil {
			handlers.OnSnapshotPrepare(t)
		}
	case *events.SnapshotCommit:
		if handlers.OnSnapshotCommit != nil {
			handlers.OnSnapshotCommit(t)
		}
	case *events.SnapshotRemove:
		if handlers.OnSnapshotRemove != nil {
			handlers.OnSnapshotRemove(t)
		}
	case *events.NamespaceCreate:
		if handlers.OnNamespaceCreate != nil {
			handlers.OnNamespaceCreate(t)
		}
	case *events.NamespaceUpdate:
		if handlers.OnNamespaceUpdate != nil {
			handlers.OnNamespaceUpdate(t)
		}
	case *events.NamespaceDelete:
		if handlers.OnNamespaceDelete != nil {
			handlers.OnNamespaceDelete(t)
		}
	case *events.ImageCreate:
		if handlers.OnImageCreate != nil {
			handlers.OnImageCreate(t)
		}
	case *events.ImageUpdate:
		if handlers.OnImageUpdate != nil {
			handlers.OnImageUpdate(t)
		}
	case *events.ImageDelete:
		if handlers.OnImageDelete != nil {
			handlers.OnImageDelete(t)
		}
	case *events.ContentCreate:
		if handlers.OnContentCreate != nil {
			handlers.OnContentCreate(t)
		}
	case *events.ContentDelete:
		if handlers.OnContentDelete != nil {
			handlers.OnContentDelete(t)
		}
	case *events.ContainerCreate:
		if handlers.OnContainerCreate != nil {
			handlers.OnContainerCreate(t)
		}
	case *events.ContainerUpdate:
		if handlers.OnContainerUpdate != nil {
			handlers.OnContainerUpdate(t)
		}
	case *events.ContainerDelete:
		if handlers.OnContainerDelete != nil {
			handlers.OnContainerDelete(t)
		}
	default:
		c.logger.Warn("no handler exists for event", "event", fmt.Sprintf("%s", t))
	}
}

// onTaskExitHandler is run whenever a container task exits. This is usually a good opportunity to perform
// cleanup tasks because the task is not yet removed. See deleteHandler for handling deletions.
func (c *Controller) onTaskExitHandler(e *events.TaskExit) {
	ctx := context.Background()
	c.logger.Debug("runtime task exited", "task", e.GetContainerID(), "status", e.GetExitStatus(), "pid", e.GetPid())
	err := c.exchange.Publish(ctx, voiyd_events.NewEvent(voiyd_events.RuntimeTaskExit, e))
	if err != nil {
		c.logger.Error("error emitting event", "error", err, "task", e.GetContainerID(), "eventType", e.String())
	}
}

func (c *Controller) onTaskCreateHandler(e *events.TaskCreate) {
	ctx := context.Background()
	c.logger.Debug("runtime task created", "task", e.GetContainerID(), "pid", e.GetPid())
	err := c.exchange.Publish(ctx, voiyd_events.NewEvent(voiyd_events.RuntimeTaskCreate, e))
	if err != nil {
		c.logger.Error("error emitting event", "error", err, "task", e.GetContainerID(), "eventType", e.String())
	}
}

func (c *Controller) onContainerCreateHandler(e *events.ContainerCreate) {
	ctx := context.Background()
	c.logger.Debug("runtime container created", "container", e.GetID(), "image", e.GetImage(), "runtime", e.GetRuntime())
	err := c.exchange.Publish(ctx, voiyd_events.NewEvent(voiyd_events.RuntimeContainerCreate, e))
	if err != nil {
		c.logger.Error("error emitting event", "error", err, "task", e.GetID(), "eventType", e.String())
	}
}

func (c *Controller) onTaskStartHandler(e *events.TaskStart) {
	ctx := context.Background()
	c.logger.Debug("runtime task started", "task", e.GetContainerID(), "pid", e.GetPid())
	err := c.exchange.Publish(ctx, voiyd_events.NewEvent(voiyd_events.RuntimeTaskStart, e))
	if err != nil {
		c.logger.Error("error emitting event", "error", err, "task", e.GetContainerID(), "eventType", e.String())
	}
}

func (c *Controller) onTaskDeleteHandler(e *events.TaskDelete) {
	ctx := context.Background()
	c.logger.Debug("runtime task deleted", "task", e.GetContainerID(), "pid", e.GetPid(), "status", e.GetExitStatus())
	err := c.exchange.Publish(ctx, voiyd_events.NewEvent(voiyd_events.RuntimeTaskDelete, e))
	if err != nil {
		c.logger.Error("error emitting event", "error", err, "task", e.GetContainerID(), "eventType", e.String())
	}
}

func (c *Controller) onTaskIOHandler(e *events.TaskIO) {
	ctx := context.Background()
	c.logger.Debug("runtime IO in progress", "stdout", e.GetStdout(), "stderr", e.GetStderr(), "stdin", e.GetStdin())
	err := c.exchange.Publish(ctx, voiyd_events.NewEvent(voiyd_events.RuntimeTaskIO, e))
	if err != nil {
		c.logger.Error("error emitting event", "error", err, "eventType", e.String())
	}
}

func (c *Controller) onTaskOOMHandler(e *events.TaskOOM) {
	ctx := context.Background()
	c.logger.Debug("runtime task out-of-memory (OOM)", "task", e.GetContainerID())
	err := c.exchange.Publish(ctx, voiyd_events.NewEvent(voiyd_events.RuntimeTaskOOM, e))
	if err != nil {
		c.logger.Error("error emitting event", "error", err, "task", e.GetContainerID(), "eventType", e.String())
	}
}

func (c *Controller) onTaskExecAddedHandler(e *events.TaskExecAdded) {
	ctx := context.Background()
	c.logger.Debug("runtime task exec added", "task", e.GetContainerID(), "execID", e.GetExecID())
	err := c.exchange.Publish(ctx, voiyd_events.NewEvent(voiyd_events.RuntimeTaskExecAdded, e))
	if err != nil {
		c.logger.Error("error emitting event", "error", err, "task", e.GetContainerID(), "eventType", e.String())
	}
}

func (c *Controller) onTaskExecStartedHandler(e *events.TaskExecStarted) {
	ctx := context.Background()
	c.logger.Debug("runtime task exec started", "task", e.GetContainerID(), "execID", e.GetExecID(), "pid", e.GetPid())
	err := c.exchange.Publish(ctx, voiyd_events.NewEvent(voiyd_events.RuntimeTaskExecStarted, e))
	if err != nil {
		c.logger.Error("error emitting event", "error", err, "task", e.GetContainerID(), "eventType", e.String())
	}
}

func (c *Controller) onTaskPausedHandler(e *events.TaskPaused) {
	ctx := context.Background()
	c.logger.Debug("runtime task paused", "task", e.GetContainerID())
	err := c.exchange.Publish(ctx, voiyd_events.NewEvent(voiyd_events.RuntimeTaskPaused, e))
	if err != nil {
		c.logger.Error("error emitting event", "error", err, "task", e.GetContainerID(), "eventType", e.String())
	}
}

func (c *Controller) onTaskResumedHandler(e *events.TaskResumed) {
	ctx := context.Background()
	c.logger.Debug("runtime task resumed", "task", e.GetContainerID())
	err := c.exchange.Publish(ctx, voiyd_events.NewEvent(voiyd_events.RuntimeTaskResumed, e))
	if err != nil {
		c.logger.Error("error emitting event", "error", err, "task", e.GetContainerID(), "eventType", e.String())
	}
}

func (c *Controller) onTaskCeckpointedHandler(e *events.TaskCheckpointed) {
	ctx := context.Background()
	c.logger.Debug("runtime task checkpointed", "task", e.GetContainerID(), "checkpoint", e.GetCheckpoint())
	err := c.exchange.Publish(ctx, voiyd_events.NewEvent(voiyd_events.RuntimeTaskCheckpointed, e))
	if err != nil {
		c.logger.Error("error emitting event", "error", err, "task", e.GetContainerID(), "eventType", e.String())
	}
}

func (c *Controller) onImageCreateHandler(e *events.ImageCreate) {
	ctx := context.Background()
	c.logger.Debug("runtime image created", "image", e.GetName())
	err := c.exchange.Publish(ctx, voiyd_events.NewEvent(voiyd_events.RuntimeImageCreate, e))
	if err != nil {
		c.logger.Error("error emitting event", "error", err, "image", e.GetName(), "eventType", e.String())
	}
}

func (c *Controller) onSnapshotPrepareHandler(e *events.SnapshotPrepare) {
	ctx := context.Background()
	c.logger.Debug("runtime snapshot prepared", "snapshotter", e.GetSnapshotter())
	err := c.exchange.Publish(ctx, voiyd_events.NewEvent(voiyd_events.RuntimeSnapshotPrepare, e))
	if err != nil {
		c.logger.Error("error emitting event", "error", err, "snapshotter", e.GetSnapshotter(), "eventType", e.String())
	}
}

func (c *Controller) onSnapshotCommitHandler(e *events.SnapshotCommit) {
	ctx := context.Background()
	c.logger.Debug("runtime snapshot committed", "snapshotter", e.GetSnapshotter())
	err := c.exchange.Publish(ctx, voiyd_events.NewEvent(voiyd_events.RuntimeSnapshotCommit, e))
	if err != nil {
		c.logger.Error("error emitting event", "error", err, "snapshotter", e.GetSnapshotter(), "eventType", e.String())
	}
}

func (c *Controller) onSnapshotRemoveHandler(e *events.SnapshotRemove) {
	ctx := context.Background()
	c.logger.Debug("runtime snapshot removed", "snapshotter", e.GetSnapshotter())
	err := c.exchange.Publish(ctx, voiyd_events.NewEvent(voiyd_events.RuntimeSnapshotRemove, e))
	if err != nil {
		c.logger.Error("error emitting event", "error", err, "snapshotter", e.GetSnapshotter(), "eventType", e.String())
	}
}

func (c *Controller) onContentCreateHandler(e *events.ContentCreate) {
	ctx := context.Background()
	c.logger.Debug("runtime snapshot committed", "size", e.GetSize(), "digest", e.GetDigest())
	err := c.exchange.Publish(ctx, voiyd_events.NewEvent(voiyd_events.RuntimeContentCreate, e))
	if err != nil {
		c.logger.Error("error emitting event", "error", err, "size", e.GetSize(), "digest", e.GetDigest(), "eventType", e.String())
	}
}

func (c *Controller) onContentDeleteHandler(e *events.ContentDelete) {
	ctx := context.Background()
	c.logger.Debug("runtime content deleted", "digest", e.GetDigest())
	err := c.exchange.Publish(ctx, voiyd_events.NewEvent(voiyd_events.RuntimeContentDelete, e))
	if err != nil {
		c.logger.Error("error emitting event", "error", err, "digest", e.GetDigest(), "eventType", e.String())
	}
}

// Reconcile ensures that desired containers matches with containers
// in the runtime environment. It removes any containers that are not
// desired (missing from the server) and adds those missing from runtime.
// It is preferrably run early during startup of the controller.
func (c *Controller) Reconcile(ctx context.Context) error {
	return nil
}

func New(client *containerd.Client, rt runtime.Runtime, opts ...NewOption) *Controller {
	eh := &Controller{
		client:  client,
		runtime: rt,
		logger:  logger.ConsoleLogger{},
	}

	handlers := &RuntimeHandlerFuncs{
		OnTaskExit:         eh.onTaskExitHandler,
		OnTaskCreate:       eh.onTaskCreateHandler,
		OnTaskStart:        eh.onTaskStartHandler,
		OnTaskDelete:       eh.onTaskDeleteHandler,
		OnTaskIO:           eh.onTaskIOHandler,
		OnTaskOOM:          eh.onTaskOOMHandler,
		OnTaskExecAdded:    eh.onTaskExecAddedHandler,
		OnTaskExecStarted:  eh.onTaskExecStartedHandler,
		OnTaskPaused:       eh.onTaskPausedHandler,
		OnTaskResumed:      eh.onTaskResumedHandler,
		OnTaskCheckpointed: eh.onTaskCeckpointedHandler,
		OnContainerCreate:  eh.onContainerCreateHandler,
		OnImageCreate:      eh.onImageCreateHandler,
		OnSnapshotPrepare:  eh.onSnapshotPrepareHandler,
		OnSnapshotCommit:   eh.onSnapshotCommitHandler,
		OnSnapshotRemove:   eh.onSnapshotRemoveHandler,
		OnContentCreate:    eh.onContentCreateHandler,
		OnContentDelete:    eh.onContentDeleteHandler,
	}

	eh.handlers = handlers

	for _, opt := range opts {
		opt(eh)
	}
	return eh
}
