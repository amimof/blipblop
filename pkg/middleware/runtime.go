package middleware

import (
	"context"
	"fmt"
	"log"

	"github.com/amimof/blipblop/api/services/containers/v1"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/amimof/blipblop/pkg/informer"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/api/events"
	"github.com/containerd/containerd/namespaces"
	gocni "github.com/containerd/go-cni"
)

type runtimeMiddleware struct {
	informer *informer.RuntimeInformer
	client   *client.Client
	runtime  *client.RuntimeClient
}

func (r *runtimeMiddleware) exitHandler(e *events.TaskExit) {
	err := r.setContainerState(e.ContainerID)
	if err != nil {
		log.Printf("%s: %s - error setting container state: %s", e.ContainerID, "TaskExit", err)
	}
}

func (r *runtimeMiddleware) createHandler(e *events.TaskCreate) {
	err := r.setContainerState(e.ContainerID)
	if err != nil {
		log.Printf("%s: %s - error setting container state: %s", e.ContainerID, "TaskCreate", err)
	}
}

func (r *runtimeMiddleware) containerCreateHandler(e *events.ContainerCreate) {
	ns := "blipblop"
	ctx := context.Background()
	ctx = namespaces.WithNamespace(ctx, ns)
	err := r.client.SetContainerState(ctx, e.ID, "created")
	if err != nil {
		log.Printf("%s: %s - error setting container state: %s", e.ID, "ContainerCreate", err)
	}
	err = r.client.SetContainerNode(ctx, e.ID, r.client.Name())
	if err != nil {
		log.Printf("%s: %s - error setting container node: %s", e.ID, "ContainerCreate", err)
	}
}

func (r *runtimeMiddleware) startHandler(e *events.TaskStart) {
	err := r.setContainerState(e.ContainerID)
	if err != nil {
		log.Printf("%s: %s - error setting container state: %s", e.ContainerID, "TaskStart", err)
	}
}

// TODO: Consider not updating container state on delete events since the container is already gone.
// In other words, there will never be anything to update after a delete event. Keeping this as-is for verbosity
func (r *runtimeMiddleware) deleteHandler(e *events.TaskDelete) {
	err := r.setContainerState(e.ContainerID)
	if err != nil {
		log.Printf("%s: %s - error setting container state: %s", e.ContainerID, "TaskDelete", err)
	}
}
func (r *runtimeMiddleware) ioHandler(e *events.TaskIO) {
}
func (r *runtimeMiddleware) oomHandler(e *events.TaskOOM) {
	err := r.setContainerState(e.ContainerID)
	if err != nil {
		log.Printf("%s: %s - error setting container state: %s", e.ContainerID, "TaskOOM", err)
	}
}
func (r *runtimeMiddleware) execAddedHandler(e *events.TaskExecAdded) {
	err := r.setContainerState(e.ContainerID)
	if err != nil {
		log.Printf("%s: %s - error setting container state: %s", e.ContainerID, "TaskExecAdded", err)
	}
}
func (r *runtimeMiddleware) execStartedHandler(e *events.TaskExecStarted) {
	err := r.setContainerState(e.ContainerID)
	if err != nil {
		log.Printf("%s: %s - error setting container state: %s", e.ContainerID, "TaskExecStarted", err)
	}
}
func (r *runtimeMiddleware) pausedHandler(e *events.TaskPaused) {
	err := r.setContainerState(e.ContainerID)
	if err != nil {
		log.Printf("%s: %s - error setting container state: %s", e.ContainerID, "TaskPaused", err)
	}
}
func (r *runtimeMiddleware) resumedHandler(e *events.TaskResumed) {
	err := r.setContainerState(e.ContainerID)
	if err != nil {
		log.Printf("%s: %s - error setting container state: %s", e.ContainerID, "TaskResumed", err)
	}
}
func (r *runtimeMiddleware) checkpointedHandler(e *events.TaskCheckpointed) {
	err := r.setContainerState(e.ContainerID)
	if err != nil {
		log.Printf("%s: %s - error setting container state: %s", e.ContainerID, "TaskCheckpointed", err)
	}
}

func (r *runtimeMiddleware) setContainerState(id string) error {
	ns := "blipblop"
	ctx := context.Background()
	ctx = namespaces.WithNamespace(ctx, ns)
	list, err := r.runtime.ContainerdClient().Containers(ctx)
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
			err = r.client.SetContainerState(ctx, id, fmt.Sprintf("%s", status.Status))
			if err != nil {
				return err
			}
			return nil
		}
	}
	return nil
}

func (r *runtimeMiddleware) Run(ctx context.Context, stop <-chan struct{}) {
	err := r.Recouncile(ctx)
	if err != nil {
		log.Printf("Error recounciling state with error %s", err)
		return
	}
	go r.informer.Watch(ctx, stop)
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
func (r *runtimeMiddleware) Recouncile(ctx context.Context) error {
	// Get containers from the server. Ultimately we want these to match with our runtime
	containers, err := r.client.ListContainers(ctx)
	if err != nil {
		return err
	}

	// Get containers from our runtime
	currentContainers, err := r.runtime.List(ctx)
	if err != nil {
		return err
	}

	// Check if there are containers in our runtime that doesn't exist on the server.
	for _, c := range currentContainers {
		if !contains(containers, c) {
			err := r.runtime.Stop(ctx, c.Name)
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
	for _, c := range containers {
		if !contains(currentContainers, c) {
			err := r.runtime.Set(ctx, c)
			if err != nil {
				log.Printf("error creating container: %s", err)
			}
		}
	}
	return nil
}

func WithRuntime(c *client.Client, cc *containerd.Client, cni gocni.CNI) Middleware {
	m := &runtimeMiddleware{
		client:  c,
		runtime: client.NewContainerdRuntimeClient(cc, cni),
	}
	i := informer.NewRuntimeInformer(m.runtime.ContainerdClient())
	i.AddHandler(&informer.RuntimeHandlerFuncs{
		OnTaskExit:         m.exitHandler,
		OnTaskCreate:       m.createHandler,
		OnTaskStart:        m.startHandler,
		OnTaskDelete:       m.deleteHandler,
		OnTaskIO:           m.ioHandler,
		OnTaskOOM:          m.oomHandler,
		OnTaskExecAdded:    m.execAddedHandler,
		OnTaskExecStarted:  m.execStartedHandler,
		OnTaskPaused:       m.pausedHandler,
		OnTaskResumed:      m.resumedHandler,
		OnTaskCheckpointed: m.checkpointedHandler,
		OnContainerCreate:  m.containerCreateHandler,
	})
	m.informer = i
	return m
}
