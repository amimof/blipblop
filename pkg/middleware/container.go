package middleware

import (
	//"os"
	"context"
	"github.com/amimof/blipblop/internal/repo"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/amimof/blipblop/pkg/informer"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/api/events"
	gocni "github.com/containerd/go-cni"
	"log"
)

type containerMiddleware struct {
	informer *informer.RuntimeInformer
	repo     repo.ContainerRepo
	client   *client.Client
	runtime  *client.RuntimeClient
}

//
func (c *containerMiddleware) exitHandler(e *events.TaskExit) {
	ctx := context.Background()
	err := c.repo.Kill(ctx, e.ID)
	if err != nil {
		log.Printf("Got an error while trying to kill task %s. The error was %s", e.ID, err.Error())
	}
	err = c.repo.Start(ctx, e.ContainerID)
	log.Printf("Task %s just exited", e.ID)
}

func (c *containerMiddleware) createHandler(e *events.TaskCreate) {
	log.Printf("Task was just created with PID %d", e.Pid)
}

func (c *containerMiddleware) Run(ctx context.Context, stop <-chan struct{}) {
	go c.informer.Watch(ctx, stop)
}

func WithRuntime(c *client.Client, cc *containerd.Client, cni gocni.CNI) Middleware {
	m := &containerMiddleware{
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
