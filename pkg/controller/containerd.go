package controller

import (
	//"os"
	"context"
	"log"

	"github.com/amimof/blipblop/api/services/containers/v1"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/amimof/blipblop/pkg/runtime"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/api/events"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/typeurl"
)

type ContainerdController struct {
	client    *containerd.Client
	clientset *client.ClientSet
	handlers  *RuntimeHandlerFuncs
	runtime   runtime.Runtime
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

func (i *ContainerdController) Run(ctx context.Context, stopCh <-chan struct{}) {
	err := i.Recouncile(ctx)
	if err != nil {
		log.Printf("Error recounciling state with error %s", err)
		return
	}
	ch, errs := i.client.Subscribe(ctx)
	for {
		select {
		case e := <-ch:
			ev, err := typeurl.UnmarshalAny(e.Event)
			if err != nil {
				log.Printf("Error: %s", err.Error())
			}
			i.HandleEvent(i.handlers, ev)
		case err := <-errs:
			if err != nil {
				log.Printf("Error %s", err)
			}
		case <-ctx.Done():
			return
		case <-stopCh:
			i.client.Close()
			ctx.Done()
		}
	}
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
		log.Printf("No handler exists for event %s", t)
	}
}

func (r *ContainerdController) exitHandler(e *events.TaskExit) {
	err := r.setContainerState(e.ContainerID)
	if err != nil {
		log.Printf("%s: %s - error setting container state: %s", e.ContainerID, "TaskExit", err)
	}
}

func (r *ContainerdController) createHandler(e *events.TaskCreate) {
	err := r.setContainerState(e.ContainerID)
	if err != nil {
		log.Printf("%s: %s - error setting container state: %s", e.ContainerID, "TaskCreate", err)
	}
}

func (r *ContainerdController) containerCreateHandler(e *events.ContainerCreate) {
	ns := "blipblop"
	ctx := context.Background()
	ctx = namespaces.WithNamespace(ctx, ns)
	err := r.clientset.ContainerV1().SetContainerState(ctx, e.ID, "created")
	if err != nil {
		log.Printf("%s: %s - error setting container state: %s", e.ID, "ContainerCreate", err)
	}
	err = r.clientset.ContainerV1().SetContainerNode(ctx, e.ID, r.clientset.Name())
	if err != nil {
		log.Printf("%s: %s - error setting container node: %s", e.ID, "ContainerCreate", err)
	}
}

func (r *ContainerdController) startHandler(e *events.TaskStart) {
	err := r.setContainerState(e.ContainerID)
	if err != nil {
		log.Printf("%s: %s - error setting container state: %s", e.ContainerID, "TaskStart", err)
	}
}

// TODO: Consider not updating container state on delete events since the container is already gone.
// In other words, there will never be anything to update after a delete event. Keeping this as-is for verbosity
func (r *ContainerdController) deleteHandler(e *events.TaskDelete) {
	err := r.setContainerState(e.ContainerID)
	if err != nil {
		log.Printf("%s: %s - error setting container state: %s", e.ContainerID, "TaskDelete", err)
	}
}

func (r *ContainerdController) ioHandler(e *events.TaskIO) {
}

func (r *ContainerdController) oomHandler(e *events.TaskOOM) {
	err := r.setContainerState(e.ContainerID)
	if err != nil {
		log.Printf("%s: %s - error setting container state: %s", e.ContainerID, "TaskOOM", err)
	}
}

func (r *ContainerdController) execAddedHandler(e *events.TaskExecAdded) {
	err := r.setContainerState(e.ContainerID)
	if err != nil {
		log.Printf("%s: %s - error setting container state: %s", e.ContainerID, "TaskExecAdded", err)
	}
}

func (r *ContainerdController) execStartedHandler(e *events.TaskExecStarted) {
	err := r.setContainerState(e.ContainerID)
	if err != nil {
		log.Printf("%s: %s - error setting container state: %s", e.ContainerID, "TaskExecStarted", err)
	}
}

func (r *ContainerdController) pausedHandler(e *events.TaskPaused) {
	err := r.setContainerState(e.ContainerID)
	if err != nil {
		log.Printf("%s: %s - error setting container state: %s", e.ContainerID, "TaskPaused", err)
	}
}

func (r *ContainerdController) resumedHandler(e *events.TaskResumed) {
	err := r.setContainerState(e.ContainerID)
	if err != nil {
		log.Printf("%s: %s - error setting container state: %s", e.ContainerID, "TaskResumed", err)
	}
}

func (r *ContainerdController) checkpointedHandler(e *events.TaskCheckpointed) {
	err := r.setContainerState(e.ContainerID)
	if err != nil {
		log.Printf("%s: %s - error setting container state: %s", e.ContainerID, "TaskCheckpointed", err)
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
			task, err := container.Task(ctx, nil)
			if err != nil {
				return err
			}
			status, err := task.Status(ctx)
			if err != nil {
				return err
			}
			err = r.clientset.ContainerV1().SetContainerState(ctx, id, string(status.Status))
			if err != nil {
				return err
			}
			return nil
		}
	}
	return nil
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
func (r *ContainerdController) Recouncile(ctx context.Context) error {
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
			err := r.runtime.Kill(ctx, c.Name)
			if err != nil {
				log.Printf("error stopping container: %s", err)
			}
			err = r.runtime.Delete(ctx, c.Name)
			if err != nil {
				log.Printf("error deleting container: %s", err)
			}
		}
	}

	// Check if  there are containers on the server that doesn't exist in our runtime
	for _, c := range clist {
		if !contains(currentContainers, c) {
			err := r.runtime.Create(ctx, c)
			if err != nil {
				log.Printf("error creating container: %s", err)
			}
		}
	}
	return nil
}

func NewContainerdController(client *containerd.Client, cs *client.ClientSet, rt runtime.Runtime) *ContainerdController {
	eh := &ContainerdController{
		client:    client,
		clientset: cs,
		runtime:   rt,
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

	return eh
}
