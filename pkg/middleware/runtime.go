package middleware

import (
	"context"
	"github.com/amimof/blipblop/internal/models"
	"github.com/amimof/blipblop/internal/repo"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/amimof/blipblop/pkg/informer"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/api/events"
	gocni "github.com/containerd/go-cni"
	"log"
)

type runtimeMiddleware struct {
	informer *informer.RuntimeInformer
	repo     repo.ContainerRepo
	client   *client.Client
	runtime  *client.RuntimeClient
}

//
func (r *runtimeMiddleware) exitHandler(e *events.TaskExit) {
	// ctx := context.Background()
	// err := r.repo.Kill(ctx, e.ID)
	// if err != nil {
	// 	log.Printf("Got an error while trying to kill task %s. The error was %s", e.ID, err.Error())
	// }
	// err = r.repo.Start(ctx, e.ContainerID)
	log.Printf("Task %s just exited", e.ID)
}

func (r *runtimeMiddleware) createHandler(e *events.TaskCreate) {
	log.Printf("Task was just created with PID %d", e.Pid)
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
func contains(cs []*models.Container, c *models.Container) bool {
	for _, container := range cs {
		if container.Revision == c.Revision && *container.Name == *c.Name {
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
	currentContainers, err := r.runtime.GetAll(ctx)
	if err != nil {
		return err
	}

	// Check if there are containers in our runtime that doesn't exist on the server.
	for _, c := range currentContainers {
		if !contains(containers, c) {
			r.runtime.Kill(ctx, *c.Name)
			err := r.runtime.Delete(ctx, *c.Name)
			if err != nil {
				log.Printf("error removing container: %s", err)
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
		OnTaskExit:   m.exitHandler,
		OnTaskCreate: m.createHandler,
	})
	m.informer = i
	return m
}
