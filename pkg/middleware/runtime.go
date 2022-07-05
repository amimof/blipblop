package middleware

import (
	//"os"
	"context"
	"github.com/amimof/blipblop/internal/models"
	"github.com/amimof/blipblop/internal/repo"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/amimof/blipblop/pkg/informer"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/api/events"
	gocni "github.com/containerd/go-cni"
	"log"
	"reflect"
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
		if reflect.DeepEqual(container, c) {
			return true
		}
	}
	return false
}

func (r *runtimeMiddleware) Recouncile(ctx context.Context) error {
	var toremove []*models.Container
	var toadd []*models.Container
	//var toupdate []*models.Container

	containers, err := r.client.ListContainers(ctx)
	if err != nil {
		return err
	}
	log.Printf("Got %d containers from server", len(containers))

	currentContainers, err := r.runtime.GetAll(ctx)
	if err != nil {
		return err
	}
	log.Printf("Got %d containers from runtime env", len(currentContainers))

	for _, c := range currentContainers {
		if !contains(containers, c) {
			toremove = append(toremove, c)
		}
	}
	log.Printf("Going to remove %d from runtime env", len(toremove))

	for _, c := range containers {
		if !contains(currentContainers, c) {
			toadd = append(toadd, c)
		}
	}
	log.Printf("Going to add %d to runtime env", len(toremove))

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
