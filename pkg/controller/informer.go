package controller

import (
	//"os"
	"log"
	"context"
	"github.com/containerd/typeurl"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/api/events"
)

type Informer struct {
	client *containerd.Client
	handlers *HandlerFuncs
}

type HandlerFuncs struct {
	OnTaskExit func(obj *events.TaskExit)
	OnTaskCreate func(obj *events.TaskCreate)
}

func (i *Informer) AddHandler(h *HandlerFuncs) {
	i.handlers = h
}

func (i *Informer) Watch(stopCh <-chan struct{}) {
	ch, errs := i.client.Subscribe(context.Background())
	for {	
		select {
		case e := <-ch:
			ev, err := typeurl.UnmarshalAny(e.Event)
			if err != nil {
				log.Printf("Error: %s", err.Error())
			}
			handleEvent(i.handlers, ev)
		case err := <-errs:
			log.Printf("Error %s", err.Error())
		case <-stopCh:
			log.Printf("Stopping informer...")
		}
	}
	log.Printf("Stopped watching informer")
}

func handleEvent(handlers *HandlerFuncs, obj interface{}) {
	switch t := obj.(type) {
	case *events.TaskExit:
		handlers.OnTaskExit(t)
	case *events.TaskCreate:
		handlers.OnTaskCreate(t)
	default:
		log.Printf("Unknown event type")
	}
}

func NewInformer(client *containerd.Client) *Informer {
	return &Informer{
		client: client,
	}
}


