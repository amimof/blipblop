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
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/api/events"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/namespaces"
	typeurl "github.com/containerd/typeurl/v2"
	"google.golang.org/protobuf/types/known/timestamppb"
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
	OnContentDelete    func(*events.ContentDelete)
	OnContainerCreate  func(*events.ContainerCreate)
	OnContainerUpdate  func(*events.ContainerUpdate)
	OnContainerDelete  func(*events.ContainerDelete)
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
			c.client.Close()
			c.clientset.Close()
		}
	}
}

func (c *ContainerdController) HandleEvent(handlers *RuntimeHandlerFuncs, obj interface{}) {
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
		c.logger.Warn("no handler exists for event", "event", fmt.Sprintf("%s", t))
	}
}

// TODO: Experimental, might remove later
func (c *ContainerdController) teardownNetworkForContainer(id string) error {
	ctx := context.Background()
	ctr, err := c.clientset.ContainerV1().Get(ctx, id)
	if err != nil {
		return err
	}
	err = c.runtime.Cleanup(context.Background(), ctr)
	if err != nil {
		return err
	}
	return nil
}

func (c *ContainerdController) exitHandler(e *events.TaskExit) {
	err := c.setContainerState(e.ID)
	if err != nil {
		c.logger.Error("error setting container state", "id", e.ID, "event", "ContainerCreate", "error", err)
	}
	// ctx := namespaces.WithNamespace(context.Background(), c.runtime.Namespace())
	//
	// id := e.ContainerID
	// err := c.setContainerState(id)
	// if err != nil {
	// 	c.logger.Error("error setting container state", "id", id, "event", "TaskExit", "error", err)
	// }
	//
	// ctr, err := c.clientset.ContainerV1().Get(ctx, e.ContainerID)
	// if err != nil {
	// 	c.logger.Error("error getting container", "error", err, "containerID", e.ContainerID)
	// 	return
	// }

	// _ = c.clientset.ContainerV1().SetTaskStatus(ctx, ctr.GetMeta().GetName(), containersv1.Phase_Deleting.String())
	// err = c.runtime.Delete(ctx, ctr)
	// if err != nil {
	// 	c.logger.Error("error deleting container", "id", id, "error", err)
	// 	return
	// }
	//
	// err = c.runtime.Cleanup(ctx, ctr)
	// if err != nil {
	// 	c.logger.Error("error running garbage collector in runtime for container", "id", id, "error", err)
	// 	return
	// }
	// err = c.teardownNetworkForContainer(id)
	// if err != nil {
	// 	c.logger.Error("error running garbage collector in runtime for container", "id", id, "error", err)
	// }
}

func (c *ContainerdController) createHandler(e *events.TaskCreate) {
	err := c.setContainerState(e.ContainerID)
	if err != nil {
		c.logger.Error("error setting container state", "id", e.ContainerID, "event", "TaskCreate", "error", err)
	}
}

func (c *ContainerdController) containerCreateHandler(e *events.ContainerCreate) {
	err := c.setContainerState(e.ID)
	if err != nil {
		c.logger.Error("error setting container state", "id", e.ID, "event", "ContainerCreate", "error", err)
	}
}

func (c *ContainerdController) startHandler(e *events.TaskStart) {
	err := c.setContainerState(e.ContainerID)
	if err != nil {
		c.logger.Error("error setting container state", "id", e.ContainerID, "event", "TaskStart", "error", err)
	}
}

// TODO: Consider not updating container state on delete events since the container is already gone.
// In other words, there will never be anything to update after a delete event. Keeping this as-is for verbosity
func (c *ContainerdController) deleteHandler(e *events.TaskDelete) {
	id := e.ContainerID
	err := c.setContainerState(id)
	if err != nil {
		c.logger.Error("error setting container state", "id", id, "event", "TaskDelete", "error", err)
	}
	err = c.teardownNetworkForContainer(id)
	if err != nil {
		c.logger.Error("error running garbage collector in runtime for container", "id", id, "error", err)
	}
}

func (c *ContainerdController) ioHandler(e *events.TaskIO) {
}

func (c *ContainerdController) oomHandler(e *events.TaskOOM) {
	err := c.setContainerState(e.ContainerID)
	if err != nil {
		c.logger.Error("error setting container state", "id", e.ContainerID, "event", "TaskOOM", "error", err)
	}
}

func (c *ContainerdController) execAddedHandler(e *events.TaskExecAdded) {
	err := c.setContainerState(e.ContainerID)
	if err != nil {
		c.logger.Error("error setting container state", "id", e.ContainerID, "event", "TaskExecAdded", "error", err)
	}
}

func (c *ContainerdController) execStartedHandler(e *events.TaskExecStarted) {
	err := c.setContainerState(e.ContainerID)
	if err != nil {
		c.logger.Error("error setting container state", "id", e.ContainerID, "event", "TaskExecStarted", "error", err)
	}
	err = c.setTaskIOConfig(e.ContainerID)
	if err != nil {
		c.logger.Error("error setting container IO config", "id", e.ContainerID, "event", "TaskStart", "error", err)
	}
}

func (c *ContainerdController) pausedHandler(e *events.TaskPaused) {
	err := c.setContainerState(e.ContainerID)
	if err != nil {
		c.logger.Error("error setting container state", "id", e.ContainerID, "event", "TaskPaused", "error", err)
	}
}

func (c *ContainerdController) resumedHandler(e *events.TaskResumed) {
	err := c.setContainerState(e.ContainerID)
	if err != nil {
		c.logger.Error("error setting container state", "id", e.ContainerID, "event", "TaskResumed", "error", err)
	}
	err = c.setTaskIOConfig(e.ContainerID)
	if err != nil {
		c.logger.Error("error setting container IO config", "id", e.ContainerID, "event", "TaskStart", "error", err)
	}
}

func (c *ContainerdController) checkpointedHandler(e *events.TaskCheckpointed) {
	err := c.setContainerState(e.ContainerID)
	if err != nil {
		c.logger.Error("error setting container state", "id", e.ContainerID, "event", "TaskCheckpointed", "error", err)
	}
}

func (c *ContainerdController) setContainerState(id string) error {
	// Get container from server
	ctx := context.Background()
	// ctr, err := c.clientset.ContainerV1().Get(ctx, id)
	// if err != nil {
	// 	return err
	// }

	ctx = namespaces.WithNamespace(context.Background(), c.runtime.Namespace())
	hostname, _ := os.Hostname()

	st := &containersv1.Status{
		Node:    hostname,
		Ip:      "192.168.13.123",
		Task:    &containersv1.TaskStatus{},
		Runtime: &containersv1.RuntimeStatus{},
	}

	task, err := c.getTask(id)
	if err != nil {
		if errdefs.IsNotFound(err) {
			st.Phase = "Deleted"
		}
	}

	var pid, exitStatus uint32
	var phase string
	var exitTime time.Time

	pid = getTaskPid(task)
	phase = getTaskProcessStatus(ctx, task)
	exitStatus = getTaskExitStatus(ctx, task)
	exitTime = getTaskExitTime(ctx, task)

	st.Node = hostname
	st.Phase = phase
	st.Task.Pid = wrapperspb.UInt32(pid)
	st.Task.ExitCode = wrapperspb.UInt32(exitStatus)
	st.Task.ExitTime = timestamppb.New(exitTime)

	c.logger.Debug("setting container status", "id", id, "status", st)
	return c.clientset.ContainerV1().Status(ctx, id, st)
}

func (c *ContainerdController) setTaskIOConfig(id string) error {
	ctx := context.Background()

	task, err := c.getTask(id)
	if err != nil {
		return err
	}

	rtCtr, err := c.runtime.Get(ctx, id)
	if err != nil {
		return err
	}

	stdoutPath := getTaskIOConfig(ctx, task).Stdout
	stderrPath := getTaskIOConfig(ctx, task).Stdin

	st := rtCtr.GetStatus()
	st.Runtime = &containersv1.RuntimeStatus{
		RuntimeEnv:     "",
		RuntimeVersion: "",
		StdoutPath:     stdoutPath,
		StderrPath:     stderrPath,
	}

	fmt.Println("Patsh", stdoutPath, stderrPath)
	return c.clientset.ContainerV1().Status(ctx, id, st)
}

func (c *ContainerdController) getTask(id string) (containerd.Task, error) {
	ctx := namespaces.WithNamespace(context.Background(), c.runtime.Namespace())

	ctr, err := c.client.LoadContainer(ctx, id)
	if err != nil {
		return nil, err
	}

	task, err := ctr.Task(ctx, nil)
	if err != nil {
		return nil, err
	}

	return task, nil
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

func getTaskExitTime(ctx context.Context, t containerd.Task) time.Time {
	return getTaskStatus(ctx, t).ExitTime
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

func getTaskIOConfig(_ context.Context, t containerd.Task) *cio.Config {
	c := t.IO().Config()
	return &c
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

// Recouncile ensures that desired containers matches with containers
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

			c.logger.Info("removing container from runtime since it's not expected to exist", "name", currentContainer.GetMeta().GetName())

			_ = c.clientset.ContainerV1().SetTaskStatus(ctx, currentContainer.GetMeta().GetName(), containersv1.Phase_Stopping.String())
			err := c.runtime.Kill(ctx, currentContainer)
			if err != nil {
				c.logger.Error("error stopping container", "error", err, "name", currentContainer.GetMeta().GetName())
			}

			_ = c.clientset.ContainerV1().SetTaskStatus(ctx, currentContainer.GetMeta().GetName(), containersv1.Phase_Deleting.String())
			err = c.runtime.Delete(ctx, currentContainer)
			if err != nil {
				c.logger.Error("error deleting container", "error", err, "name", currentContainer.GetMeta().GetName())
			}
		}
	}

	// Check if  there are containers on the server that doesn't exist in our runtime
	for _, container := range clist {
		if !contains(currentContainers, container) {
			c.logger.Info("creating container in runtime since it's expected to exist", "name", container.GetMeta().GetName())

			_ = c.clientset.ContainerV1().SetTaskStatus(ctx, container.GetMeta().GetName(), containersv1.Phase_Pulling.String())
			err := c.runtime.Pull(ctx, container)
			if err != nil {
				c.logger.Error("error pulling image", "error", err, "container", container.GetMeta().GetName())
			}

			_ = c.clientset.ContainerV1().SetTaskStatus(ctx, container.GetMeta().GetName(), containersv1.Phase_Starting.String())
			err = c.runtime.Run(ctx, container)
			if err != nil {
				c.logger.Error("error running container", "error", err, "name", container.GetMeta().GetName())
			}

			err = c.setContainerState(container.GetMeta().GetName())
			if err != nil {
				c.logger.Error("error setting container state", "container", container.GetMeta().GetName(), "error", err)
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
