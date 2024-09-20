package controller

import (
	//"os"
	"context"
	"fmt"
	"os"
	"time"

	"github.com/amimof/blipblop/api/services/containers/v1"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/amimof/blipblop/pkg/logger"
	"github.com/amimof/blipblop/pkg/runtime"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/api/events"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/typeurl"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	OnContentDelete    func(*events.ContentDelete)
	OnContainerCreate  func(*events.ContainerCreate)
	OnContainerUpdate  func(*events.ContainerUpdate)
	OnContainerDelete  func(*events.ContainerDelete)
}

func (i *ContainerdController) AddHandler(h *RuntimeHandlerFuncs) {
	i.handlers = h
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

func (i *ContainerdController) Run(ctx context.Context, stopCh <-chan struct{}) {
	err := i.Reconcile(ctx)
	if err != nil {
		i.logger.Error("error reconciling state", "error", err)
		return
	}
	err = i.streamEvents(ctx, stopCh)
	if err != nil {
		i.logger.Info("Reconnecting stream")
		i.client, err = reconnectWithBackoff("/run/containerd/containerd.sock", i.logger)
		if err != nil {
			i.logger.Error("error reconnection to stream", "error", err)
			_ = i.clientset.NodeV1().SetNodeReady(ctx, false)
		}
		_ = i.clientset.NodeV1().SetNodeReady(ctx, true)
	}
}

// TODO: Set node status to unhealthy whenever runtime is unavailable here.
// Therefore consider removing `IsServing()` check in node controller which really doesn't do much.
func (i *ContainerdController) streamEvents(ctx context.Context, stopCh <-chan struct{}) error {
	eventCh, errCh := i.client.Subscribe(ctx)
	for {
		select {
		case event := <-eventCh:
			ev, err := typeurl.UnmarshalAny(event.Event)
			if err != nil {
				i.logger.Error("error unmarshaling event received from stream", "error", err)
			}
			i.HandleEvent(i.handlers, ev)
		case err := <-errCh:
			if err == nil || isConnectionError(err) {
				i.logger.Error("received stream disconnect, attempting to reconnect")
				_ = i.clientset.NodeV1().SetNodeReady(ctx, false)
				return err
			}
			return err
		case <-ctx.Done():
			return nil
		case <-stopCh:
			i.client.Close()
			i.clientset.Close()
			ctx.Done()
		}
	}
}

func isConnectionError(err error) bool {
	if errdefs.IsUnavailable(err) || errdefs.IsNotFound(err) {
		return true
	}
	if st, ok := status.FromError(err); ok {
		return st.Code() == codes.Unavailable
	}
	return false
}

func (i *ContainerdController) HandleEvent(handlers *RuntimeHandlerFuncs, obj interface{}) {
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
		i.logger.Warn("No handler exists for event", "event", t)
	}
}

func (r *ContainerdController) exitHandler(e *events.TaskExit) {
	err := r.setContainerState(e.ContainerID)
	if err != nil {
		r.logger.Error("error setting container state", "id", e.ContainerID, "event", "TaskExit", "error", err)
	}
}

func (r *ContainerdController) createHandler(e *events.TaskCreate) {
	err := r.setContainerState(e.ContainerID)
	if err != nil {
		r.logger.Error("error setting container state", "id", e.ContainerID, "event", "TaskCreate", "error", err)
	}
}

func (r *ContainerdController) containerCreateHandler(e *events.ContainerCreate) {
	err := r.setContainerState(e.ID)
	if err != nil {
		r.logger.Error("error setting container state", "id", e.ID, "event", "ContainerCreate", "error", err)
	}
}

func (r *ContainerdController) startHandler(e *events.TaskStart) {
	err := r.setContainerState(e.ContainerID)
	if err != nil {
		r.logger.Error("error setting container state", "id", e.ContainerID, "event", "TaskStart", "error", err)
	}
}

// TODO: Consider not updating container state on delete events since the container is already gone.
// In other words, there will never be anything to update after a delete event. Keeping this as-is for verbosity
func (r *ContainerdController) deleteHandler(e *events.TaskDelete) {
	err := r.setContainerState(e.ContainerID)
	if err != nil {
		r.logger.Error("error setting container state", "id", e.ContainerID, "event", "TaskDelete", "error", err)
	}
}

func (r *ContainerdController) ioHandler(e *events.TaskIO) {
}

func (r *ContainerdController) oomHandler(e *events.TaskOOM) {
	err := r.setContainerState(e.ContainerID)
	if err != nil {
		r.logger.Error("error setting container state", "id", e.ContainerID, "event", "TaskOOM", "error", err)
	}
}

func (r *ContainerdController) execAddedHandler(e *events.TaskExecAdded) {
	err := r.setContainerState(e.ContainerID)
	if err != nil {
		r.logger.Error("error setting container state", "id", e.ContainerID, "event", "TaskExecAdded", "error", err)
	}
}

func (r *ContainerdController) execStartedHandler(e *events.TaskExecStarted) {
	err := r.setContainerState(e.ContainerID)
	if err != nil {
		r.logger.Error("error setting container state", "id", e.ContainerID, "event", "TaskExecStarted", "error", err)
	}
}

func (r *ContainerdController) pausedHandler(e *events.TaskPaused) {
	err := r.setContainerState(e.ContainerID)
	if err != nil {
		r.logger.Error("error setting container state", "id", e.ContainerID, "event", "TaskPaused", "error", err)
	}
}

func (r *ContainerdController) resumedHandler(e *events.TaskResumed) {
	err := r.setContainerState(e.ContainerID)
	if err != nil {
		r.logger.Error("error setting container state", "id", e.ContainerID, "event", "TaskResumed", "error", err)
	}
}

func (r *ContainerdController) checkpointedHandler(e *events.TaskCheckpointed) {
	err := r.setContainerState(e.ContainerID)
	if err != nil {
		r.logger.Error("error setting container state", "id", e.ContainerID, "event", "TaskCheckpointed", "error", err)
	}
}

func (r *ContainerdController) setContainerState(id string) error {
	ns := "blipblop"
	ctx := context.Background()
	ctx = namespaces.WithNamespace(ctx, ns)
	list, err := r.client.Containers(ctx)
	if err != nil {
		return err
	}
	for _, container := range list {
		if container.ID() == id {

			var pid, exitStatus uint32
			var phase string

			if task, err := container.Task(ctx, nil); err == nil {
				pid = getTaskPid(task)
				phase = getTaskProcessStatus(ctx, task)
				exitStatus = getTaskExitStatus(ctx, task)
			}

			hostname, _ := os.Hostname()

			err = r.clientset.ContainerV1().SetContainerNode(ctx, id, hostname)
			if err != nil {
				return err
			}
			st := &containers.Status{
				Node:       hostname,
				Ip:         "192.168.13.123",
				Pid:        pid,
				Phase:      phase,
				ExitStatus: exitStatus,
				Health:     "healthy",
			}
			return r.clientset.ContainerV1().SetContainerStatus(ctx, id, st)
		}
	}
	return nil
}

func getTaskPid(t containerd.Task) uint32 {
	pid := uint32(0)
	if t != nil {
		pid = t.Pid()
	}
	return pid
}

func getTaskProcessStatus(ctx context.Context, t containerd.Task) string {
	return string(getTaskStatus(ctx, t).Status)
}

func getTaskExitStatus(ctx context.Context, t containerd.Task) uint32 {
	return getTaskStatus(ctx, t).ExitStatus
}

func getTaskStatus(ctx context.Context, t containerd.Task) containerd.Status {
	s := containerd.Status{
		Status:     containerd.Unknown,
		ExitStatus: uint32(0),
	}
	if t != nil {
		if status, err := t.Status(ctx); err == nil {
			s = status
		}
	}
	return s
}

// Checks to see if c is present in cs
func contains(cs []*containers.Container, c *containers.Container) bool {
	for _, container := range cs {
		if container.Revision == c.Revision && container.Name == c.Name {
			return true
		}
	}
	return false
}

// Recouncile ensures that desired containers matches with containers
// in the runtime environment. It removes any containers that are not
// desired (missing from the server) and adds those missing from runtime.
// It is preferrably run early during startup of the controller.
func (r *ContainerdController) Reconcile(ctx context.Context) error {
	r.logger.Info("reconciling containers in containerd runtime")
	// Get containers from the server. Ultimately we want these to match with our runtime
	clist, err := r.clientset.ContainerV1().ListContainers(ctx)
	if err != nil {
		return err
	}

	// Get containers from containerd
	currentContainers, err := r.runtime.List(ctx)
	if err != nil {
		return err
	}

	// Check if there are containers in our runtime that doesn't exist on the server.
	for _, c := range currentContainers {
		if !contains(clist, c) {
			err := r.runtime.Kill(ctx, c)
			if err != nil {
				r.logger.Error("error stopping container", "error", err, "name", c.GetName())
			}
			err = r.runtime.Delete(ctx, c.Name)
			if err != nil {
				r.logger.Error("error deleting container", "error", err, "name", c.GetName())
			}
		}
	}

	// Check if  there are containers on the server that doesn't exist in our runtime
	for _, c := range clist {
		if !contains(currentContainers, c) {
			err := r.runtime.Create(ctx, c)
			if err != nil {
				r.logger.Error("error creating container", "error", err, "name", c.GetName())
			}
		}
	}
	return nil
}

func NewContainerdController(client *containerd.Client, cs *client.ClientSet, rt runtime.Runtime, opts ...NewContainerdControllerOption) *ContainerdController {
	eh := &ContainerdController{
		client:    client,
		clientset: cs,
		runtime:   rt,
		logger:    logger.ConsoleLogger{},
	}

	handlers := &RuntimeHandlerFuncs{
		OnTaskExit:         eh.exitHandler,
		OnTaskCreate:       eh.createHandler,
		OnTaskStart:        eh.startHandler,
		OnTaskDelete:       eh.deleteHandler,
		OnTaskIO:           eh.ioHandler,
		OnTaskOOM:          eh.oomHandler,
		OnTaskExecAdded:    eh.execAddedHandler,
		OnTaskExecStarted:  eh.execStartedHandler,
		OnTaskPaused:       eh.pausedHandler,
		OnTaskResumed:      eh.resumedHandler,
		OnTaskCheckpointed: eh.checkpointedHandler,
		OnContainerCreate:  eh.containerCreateHandler,
	}

	eh.handlers = handlers

	for _, opt := range opts {
		opt(eh)
	}
	return eh
}
