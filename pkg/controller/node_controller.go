package controller

import (
	"context"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/amimof/blipblop/proto"
	"github.com/containerd/containerd"
	"log"
)

type NodeInformer struct {
	handlers *NodeHandlerFuncs
	client   *client.Client
}

type NodeHandlerFuncs struct {
	OnContainerCreate func(obj *proto.Event)
	OnContainerDelete func(obj *proto.Event)
}

type nodeController struct {
	client   *client.Client
	runtime  *client.RuntimeClient
	informer *NodeInformer
}

func (i *NodeInformer) AddHandler(h *NodeHandlerFuncs) {
	i.handlers = h
}

func (i *NodeInformer) Watch(stopCh <-chan struct{}) {
	ctx := context.Background()
	evc, errc := i.client.Subscribe(ctx)
	for {
		select {
		case ev := <-evc:
			handleNodeEvent(i.handlers, ev)
		case err := <-errc:
			handleNodeError(err)
		case <-stopCh:
			ctx.Done()
			log.Println("Done watching node informer")
			return
		}
	}
}

func handleNodeEvent(h *NodeHandlerFuncs, ev *proto.Event) {
	t := ev.Type
	switch t {
	case proto.EventType_ContainerCreate:
		h.OnContainerCreate(ev)
	case proto.EventType_ContainerDelete:
		h.OnContainerDelete(ev)
	default:
		log.Printf("Handler not implemented for event type %s", t)
	}
}

func handleNodeError(err error) {
	log.Printf("error occurred handling error %s", err.Error())
}

func (n *nodeController) Run(stop <-chan struct{}) {
	go n.informer.Watch(stop)
}

func NewNodeInformer(client *client.Client) *NodeInformer {
	return &NodeInformer{
		client: client,
	}
}

func NewNodeController(c *client.Client, cc *containerd.Client) Controller {
	n := &nodeController{
		client:  c,
		runtime: client.NewContainerdRuntimeClient(cc, nil),
	}
	informer := NewNodeInformer(c)
	informer.AddHandler(&NodeHandlerFuncs{
		OnContainerCreate: func(obj *proto.Event) {
			log.Println("not implemented: OnContainerCreate")
		},
		OnContainerDelete: func(obj *proto.Event) {
			log.Println("not implemented: OnContainerDelete")
		},
	})
	n.informer = informer
	return n
}
