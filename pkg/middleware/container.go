package middleware

import (
	"context"

	"github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/amimof/blipblop/pkg/informer"
	"github.com/containerd/containerd"
	gocni "github.com/containerd/go-cni"
	"github.com/sirupsen/logrus"
)

type containerMiddleware struct {
	client   *client.ClientSet
	runtime  *client.RuntimeClient
	informer *informer.EventInformer
}

func (c *containerMiddleware) Run(ctx context.Context, stop <-chan struct{}) {
	go c.informer.Watch(ctx, stop)
}

func (c *containerMiddleware) onContainerCreate(obj *events.Event) {
	ctx := context.Background()
	cont, err := c.client.ContainerV1().GetContainer(ctx, obj.Id)
	if cont == nil {
		logrus.Printf("container %s not found", obj.Id)
		return
	}
	if err != nil {
		logrus.Printf("error occurred: %s", err.Error())
		return
	}
	err = c.runtime.Set(ctx, cont)
	if err != nil {
		logrus.Printf("error creating container %s with error: %s", cont.Name, err)
		return
	}
	logrus.Printf("successfully created container: %s", cont.Name)
}

func (c *containerMiddleware) onContainerDelete(obj *events.Event) {
	ctx := context.Background()
	err := c.runtime.Delete(ctx, obj.Id)
	if err != nil {
		logrus.Printf("error stopping container %s with error %s", obj.Id, err)
		return
	}
	logrus.Printf("successfully deleted container %s", obj.Id)
}

func (c *containerMiddleware) onContainerStart(obj *events.Event) {
	ctx := context.Background()
	err := c.runtime.Start(ctx, obj.Id)
	if err != nil {
		logrus.Printf("error starting container %s with error %s", obj.Id, err)
		_ = c.client.EventV1().Publish(ctx, obj.Id, events.EventType_ContainerStartError)
		return
	}
	logrus.Printf("successfully started container %s", obj.Id)
}

func (c *containerMiddleware) onContainerStop(obj *events.Event) {
	ctx := context.Background()
	err := c.runtime.Kill(ctx, obj.Id)
	if err != nil {
		logrus.Printf("error killing container %s with error %s", obj.Id, err)
		return
	}
	logrus.Printf("successfully killed container %s", obj.Id)
}

func WithEvents(c *client.ClientSet, cc *containerd.Client, cni gocni.CNI) Middleware {
	e := &containerMiddleware{
		client:  c,
		runtime: client.NewContainerdRuntimeClient(cc, cni),
	}
	i := informer.NewEventInformer(c)
	i.AddHandler(&informer.EventHandlerFuncs{
		OnContainerCreate: e.onContainerCreate,
		OnContainerDelete: e.onContainerDelete,
		OnContainerStart:  e.onContainerStart,
		OnContainerStop:   e.onContainerStop,
	})
	e.informer = i
	return e
}
