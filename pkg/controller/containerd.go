package controller

import (
	//"os"
	"context"
	"fmt"
	"os"
	"time"

	containersv1 "github.com/amimof/blipblop/api/services/containers/v1"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/amimof/blipblop/pkg/logger"
	"github.com/amimof/blipblop/pkg/runtime"
	"github.com/containerd/containerd/api/events"
	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	typeurl "github.com/containerd/typeurl/v2"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type ContainerdController struct {
	client    *containerd.Client
	clientset *client.ClientSet
	handlers  *RuntimeHandlerFuncs
	runtime   runtime.Runtime
	logger    logger.Logger
}

type NewContainerdControllerOption func(*ContainerdController)

func WithContainerdControllerLogger(l logger.Logger) NewContainerdControllerOption {
	return func(c *ContainerdController) {
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

func (c *ContainerdController) AddHandler(h *RuntimeHandlerFuncs) {
	c.handlers = h
}

func connectContainerd(address string) (*containerd.Client, error) {
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
		client, err = connectContainerd(address)
		if err == nil {
			l.Info("successfully connected to containerd", "address", address)
			return client, nil
		}

		l.Error("error reconnecting to containerd", "error", err, "retry_in", backoff)
		time.Sleep(backoff)
	}
}

func (c *ContainerdController) Run(ctx context.Context) {
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

func (c *ContainerdController) streamEvents(ctx context.Context) error {
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
			if err := c.clientset.Close(); err != nil {
				c.logger.Error("error closing clientset connection", "error", err)
			}
		}
	}
}

func (c *ContainerdController) HandleEvent(handlers *RuntimeHandlerFuncs, obj any) {
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
func (c *ContainerdController) onTaskExitHandler(e *events.TaskExit) {
	c.logger.Debug("task exited", "event", "TaskExit", "container", e.GetContainerID(), "execID", e.GetID())

	id := e.GetID()
	containerID := e.GetContainerID()
	ctx := namespaces.WithNamespace(context.Background(), c.runtime.Namespace())

	cName, err := c.runtime.Name(ctx, containerID)
	if err != nil {
		c.logger.Error("error resolving container name", "error", err, "id", containerID)
		return
	}

	// TODO: This if-clause is an intermediate fix to prevent receving events for non-init tasks.
	// For example if a user creates an exec task with tty we would otherwise get that event here.
	// This isn't bulletproof since in theory the user can give the exec-id the same value as the container id.
	// rendering this if useless and pontentially dangerous.
	if id == containerID {
		ctx := context.Background()
		ctx = namespaces.WithNamespace(ctx, c.runtime.Namespace())

		c.logger.Info("tearing down network for container", "id", containerID, "name", cName)
		err := c.runtime.Cleanup(ctx, cName)
		if err != nil {
			c.logger.Error("error running garbage collector in runtime for container", "error", err, "id", containerID, "name", cName)
		}

		// NOTE: The following update will not work if the task has exited as a result of a container delete event.
		// In that case the container is already removed from the server, rendering the update method below useless
		// and will always result in a not found error.
		st := &containersv1.Status{
			Phase:  wrapperspb.String("stopped"),
			Status: wrapperspb.String(fmt.Sprintf("exit status %d", e.GetExitStatus())),
			Id:     wrapperspb.String(containerID),
		}
		err = c.clientset.ContainerV1().Status().Update(ctx, cName, st, "phase", "status", "id")
		if err != nil {
			c.logger.Error("error setting container state", "error", err, "id", containerID, "event", "TaskExit", "name", cName)
		}
	}
}

func (c *ContainerdController) onTaskCreateHandler(e *events.TaskCreate) {
	ctx := namespaces.WithNamespace(context.Background(), c.runtime.Namespace())
	hostname, _ := os.Hostname()

	containerID := e.GetContainerID()

	cName, err := c.runtime.Name(ctx, containerID)
	if err != nil {
		c.logger.Error("error resolving container name", "error", err, "id", containerID)
		return
	}

	st := &containersv1.Status{
		Node:  wrapperspb.String(hostname),
		Phase: wrapperspb.String("created"),
		Id:    wrapperspb.String(containerID),
	}

	err = c.clientset.ContainerV1().Status().Update(ctx, cName, st, "node", "phase", "id")
	if err != nil {
		c.logger.Error("error setting container state", "error", err, "id", e.ContainerID, "name", cName, "event", "TaskCreate")
	}
}

func (c *ContainerdController) onContainerCreateHandler(e *events.ContainerCreate) {
	ctx := namespaces.WithNamespace(context.Background(), c.runtime.Namespace())

	containerID := e.GetID()

	cName, err := c.runtime.Name(ctx, containerID)
	if err != nil {
		c.logger.Error("error resolving container name", "error", err, "id", containerID)
		return
	}

	st := &containersv1.Status{
		Phase: wrapperspb.String("creating"),
		Id:    wrapperspb.String(containerID),
	}

	err = c.clientset.ContainerV1().Status().Update(ctx, cName, st, "phase", "id")
	if err != nil {
		c.logger.Error("error setting container state", "error", err, "id", e.GetID(), "name", cName, "event", "ContainerCreate")
	}
}

func (c *ContainerdController) onTaskStartHandler(e *events.TaskStart) {
	ctx := namespaces.WithNamespace(context.Background(), c.runtime.Namespace())
	containerID := e.GetContainerID()

	cName, err := c.runtime.Name(ctx, containerID)
	if err != nil {
		c.logger.Error("error resolving container name", "error", err, "id", containerID)
		return
	}

	hostname, _ := os.Hostname()

	st := &containersv1.Status{
		Node:  wrapperspb.String(hostname),
		Phase: wrapperspb.String("running"),
		Id:    wrapperspb.String(containerID),
	}

	err = c.clientset.ContainerV1().Status().Update(ctx, cName, st, "node", "phase", "id")
	if err != nil {
		c.logger.Error("error setting container state", "error", err, "id", e.ContainerID, "name", cName, "event", "TaskCreate")
	}
}

func (c *ContainerdController) onTaskDeleteHandler(e *events.TaskDelete) {
	c.logger.Debug("task deleted", "event", "TaskDelete", "container", e.GetContainerID(), "pid", e.GetID(), "exitStatus", e.GetExitStatus())
}

func (c *ContainerdController) onTaskIOHandler(e *events.TaskIO) {
	c.logger.Debug("handler not implemented", "event", "TaskIO")
}

func (c *ContainerdController) onTaskOOMHandler(e *events.TaskOOM) {
	ctx := namespaces.WithNamespace(context.Background(), c.runtime.Namespace())
	containerID := e.GetContainerID()

	cName, err := c.runtime.Name(ctx, containerID)
	if err != nil {
		c.logger.Error("error resolving container name", "error", err, "id", containerID)
		return
	}

	hostname, _ := os.Hostname()

	st := &containersv1.Status{
		Node:  wrapperspb.String(hostname),
		Phase: wrapperspb.String("oom"),
		Id:    wrapperspb.String(containerID),
	}

	err = c.clientset.ContainerV1().Status().Update(ctx, cName, st, "node", "phase", "id")
	if err != nil {
		c.logger.Error("error setting container state", "error", err, "id", e.ContainerID, "name", cName, "event", "TaskCreate")
	}
}

func (c *ContainerdController) onTaskExecAddedHandler(e *events.TaskExecAdded) {
	c.logger.Debug("task exec added", "event", "TaskExecAdded", "container", e.GetContainerID(), "execID", e.GetExecID())
}

func (c *ContainerdController) onTaskExecStartedHandler(e *events.TaskExecStarted) {
	// ctx := namespaces.WithNamespace(context.Background(), c.runtime.Namespace())

	// st := &containersv1.Status{
	// 	Phase: wrapperspb.String("running"),
	// 	Task: &containersv1.TaskStatus{
	// 		Pid: wrapperspb.UInt32(e.GetPid()),
	// 	},
	// }
	//
	// err := c.clientset.ContainerV1().Status().Update(ctx, e.ContainerID, st, "phase", "task.pid")
	// if err != nil {
	// 	c.logger.Error("error setting container state", "id", e.ContainerID, "event", "TaskExecStarted", "error", err)
	// }
}

func (c *ContainerdController) onTaskPausedHandler(e *events.TaskPaused) {
	ctx := namespaces.WithNamespace(context.Background(), c.runtime.Namespace())
	containerID := e.GetContainerID()

	cName, err := c.runtime.Name(ctx, containerID)
	if err != nil {
		c.logger.Error("error resolving container name", "error", err, "id", containerID)
		return
	}

	hostname, _ := os.Hostname()

	st := &containersv1.Status{
		Node:  wrapperspb.String(hostname),
		Phase: wrapperspb.String("paused"),
		Id:    wrapperspb.String(containerID),
	}

	err = c.clientset.ContainerV1().Status().Update(ctx, cName, st, "node", "phase", "id")
	if err != nil {
		c.logger.Error("error setting container state", "error", err, "id", e.ContainerID, "name", cName, "event", "TaskCreate")
	}
}

func (c *ContainerdController) onTaskResumedHandler(e *events.TaskResumed) {
	c.logger.Debug("handler not implemented", "event", "TaskIO")
}

func (c *ContainerdController) onTaskCeckpointedHandler(e *events.TaskCheckpointed) {
	c.logger.Debug("handler not implemented", "event", "TaskIO")
}

func (c *ContainerdController) onImageCreateHandler(e *events.ImageCreate) {
	c.logger.Debug("image created", "event", "ImageCreate", "image", e.GetName(), e.GetLabels)
}

func (c *ContainerdController) onSnapshotPrepareHandler(e *events.SnapshotPrepare) {
	c.logger.Debug("snapshot prepared", "event", "SnapshotPrepare", "snapshotter", e.GetSnapshotter())
}

func (c *ContainerdController) onSnapshotCommitHandler(e *events.SnapshotCommit) {
	c.logger.Debug("snapshot commited", "event", "SnapshotCommit", "snapshotter", e.GetSnapshotter(), "name", e.GetName())
}

func (c *ContainerdController) onSnapshotRemoveHandler(e *events.SnapshotRemove) {
	c.logger.Debug("snapshot removed", "event", "SnapshotRemove", "snapshotter", e.GetSnapshotter())
}

func (c *ContainerdController) onContentCreateHandler(e *events.ContentCreate) {
	c.logger.Debug("content created", "event", "ContentCreate", "digest", e.GetDigest(), "size", e.GetSize())
}

func (c *ContainerdController) onContentDeleteHandler(e *events.ContentDelete) {
	c.logger.Debug("content deleted", "event", "ContentDelete", "digest", e.GetDigest())
}

// Checks to see if c is present in cs
func contains(cs []*containersv1.Container, c *containersv1.Container) bool {
	for _, container := range cs {
		if container.GetMeta().GetRevision() == c.GetMeta().GetRevision() && container.GetMeta().GetName() == c.GetMeta().GetName() {
			return true
		}
	}
	return false
}

// Reconcile ensures that desired containers matches with containers
// in the runtime environment. It removes any containers that are not
// desired (missing from the server) and adds those missing from runtime.
// It is preferrably run early during startup of the controller.
func (c *ContainerdController) Reconcile(ctx context.Context) error {
	c.logger.Info("reconciling containers in containerd runtime")
	// Get containers from the server. Ultimately we want these to match with our runtime
	clist, err := c.clientset.ContainerV1().List(ctx)
	if err != nil {
		return err
	}

	// Get containers from containerd
	currentContainers, err := c.runtime.List(ctx)
	if err != nil {
		return err
	}

	// Check if there are containers in our runtime that doesn't exist on the server.
	for _, currentContainer := range currentContainers {
		if !contains(clist, currentContainer) {

			containerName := currentContainer.GetMeta().GetName()

			c.logger.Info("removing container from runtime since it's not expected to exist", "name", containerName)
			err := c.runtime.Kill(ctx, currentContainer)
			if err != nil {
				c.logger.Error("error stopping container", "error", err, "name", containerName)
			}

			err = c.runtime.Delete(ctx, currentContainer)
			if err != nil {
				c.logger.Error("error deleting container", "error", err, "name", containerName)
			}
		}
	}

	// Check if  there are containers on the server that doesn't exist in our runtime
	for _, container := range clist {
		if !contains(currentContainers, container) {

			containerName := container.GetMeta().GetName()

			c.logger.Info("creating container in runtime since it's expected to exist", "name", containerName)

			err := c.runtime.Pull(ctx, container)
			if err != nil {
				c.logger.Error("error pulling image", "error", err, "container", containerName)
			}

			err = c.runtime.Run(ctx, container)
			if err != nil {
				c.logger.Error("error running container", "error", err, "name", containerName)
			}

		}
	}

	return nil
}

func NewContainerdController(cs *client.ClientSet, client *containerd.Client, rt runtime.Runtime, opts ...NewContainerdControllerOption) *ContainerdController {
	eh := &ContainerdController{
		client:    client,
		clientset: cs,
		runtime:   rt,
		logger:    logger.ConsoleLogger{},
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
