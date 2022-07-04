package middleware

import (
	"context"
	"github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/amimof/blipblop/pkg/informer"
	"github.com/containerd/containerd"
	gocni "github.com/containerd/go-cni"
	"log"
)

type eventMiddleware struct {
	client   *client.Client
	runtime  *client.RuntimeClient
	informer *informer.EventInformer
}

func (e *eventMiddleware) Run(ctx context.Context, stop <-chan struct{}) {
	go e.informer.Watch(ctx, stop)
}

func WithEvents(c *client.Client, cc *containerd.Client, cni gocni.CNI) Middleware {
	e := &eventMiddleware{
		client:  c,
		runtime: client.NewContainerdRuntimeClient(cc, cni),
	}
	i := informer.NewEventInformer(c)
	i.AddHandler(&informer.EventHandlerFuncs{
		OnContainerCreate: func(obj *events.Event) {
			ctx := context.Background()
			cont, err := c.GetContainer(ctx, obj.Id)
			if cont == nil {
				log.Printf("container %s not found", obj.Id)
				return
			}
			if err != nil {
				log.Printf("error occurred: %s", err.Error())
				return
			}
			err = e.runtime.Set(ctx, cont)
			if err != nil {
				log.Printf("error creating container %s with error: %s", *cont.Name, err)
				return
			}
			log.Printf("successfully created container: %s", *cont.Name)
		},
		OnContainerDelete: func(obj *events.Event) {
			ctx := context.Background()
			err := c.DeleteContainer(ctx, obj.Id)
			if err != nil {
				log.Printf("error deleting container %s with error", obj.Id, err)
				return
			}
			err = e.runtime.Delete(ctx, obj.Id)
			if err != nil {
				log.Printf("error stopping container %s with error %s", obj.Id, err)
				return
			}
			log.Printf("successfully deleted container %s", obj.Id)
		},
		OnContainerStart: func(obj *events.Event) {
			ctx := context.Background()
			err := e.runtime.Start(ctx, obj.Id)
			if err != nil {
				log.Printf("error starting container %s with error %s", obj.Id, err)
				return
			}
			log.Printf("successfully started container %s", obj.Id)
		},
		OnContainerStop: func(obj *events.Event) {
			ctx := context.Background()
			err := e.runtime.Kill(ctx, obj.Id)
			if err != nil {
				log.Printf("error killing container %s with error %s", obj.Id, err)
				return
			}
			log.Printf("successfully killed container %s", obj.Id)
		},
	})
	e.informer = i
	return e
}
