package middleware

import (
	"context"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/amimof/blipblop/pkg/informer"
	"github.com/amimof/blipblop/api/services/events/v1"
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
			
			log.Println("not implemented: OnContainerCreate")
			cont, err := c.GetContainer(context.Background(), "asd")
			if err != nil {
				log.Printf("error occurred: %s", err.Error())
			}
			log.Printf("Got container: %s", cont.Image)
		},
		OnContainerDelete: func(obj *events.Event) {
			log.Println("not implemented: OnContainerDelete")
		},
	})
	e.informer = i
	return e
}
