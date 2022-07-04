package informer

import (
	//"os"
	"context"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/api/events"
	"github.com/containerd/typeurl"
	"log"
)

type RuntimeInformer struct {
	client   *containerd.Client
	handlers *RuntimeHandlerFuncs
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

func (i *RuntimeInformer) AddHandler(h *RuntimeHandlerFuncs) {
	i.handlers = h
}

func (i *RuntimeInformer) Watch(ctx context.Context, stopCh <-chan struct{}) {
	ch, errs := i.client.Subscribe(ctx)
	for {
		select {
		case e := <-ch:
			ev, err := typeurl.UnmarshalAny(e.Event)
			if err != nil {
				log.Printf("Error: %s", err.Error())
			}
			handleEvent(i.handlers, ev)
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

func handleEvent(handlers *RuntimeHandlerFuncs, obj interface{}) {
	switch t := obj.(type) {
	case *events.TaskExit:
		handlers.OnTaskExit(t)
	case *events.TaskCreate:
		handlers.OnTaskCreate(t)
	case *events.TaskStart:
		handlers.OnTaskStart(t)
	case *events.TaskDelete:
		handlers.OnTaskDelete(t)
	case *events.TaskIO:
		handlers.OnTaskIO(t)
	case *events.TaskOOM:
		handlers.OnTaskOOM(t)
	case *events.TaskExecAdded:
		handlers.OnTaskExecAdded(t)
	case *events.TaskExecStarted:
		handlers.OnTaskExecStarted(t)
	case *events.TaskPaused:
		handlers.OnTaskPaused(t)
	case *events.TaskResumed:
		handlers.OnTaskResumed(t)
	case *events.TaskCheckpointed:
		handlers.OnTaskCheckpointed(t)
	case *events.SnapshotPrepare:
		handlers.OnSnapshotPrepare(t)
	case *events.SnapshotCommit:
		handlers.OnSnapshotCommit(t)
	case *events.SnapshotRemove:
		handlers.OnSnapshotRemove(t)
	case *events.NamespaceCreate:
		handlers.OnNamespaceCreate(t)
	case *events.NamespaceUpdate:
		handlers.OnNamespaceUpdate(t)
	case *events.NamespaceDelete:
		handlers.OnNamespaceDelete(t)
	case *events.ImageCreate:
		handlers.OnImageCreate(t)
	case *events.ImageUpdate:
		handlers.OnImageUpdate(t)
	case *events.ImageDelete:
		handlers.OnImageDelete(t)
	case *events.ContentDelete:
		handlers.OnContentDelete(t)
	case *events.ContainerCreate:
		handlers.OnContainerCreate(t)
	case *events.ContainerUpdate:
		handlers.OnContainerUpdate(t)
	case *events.ContainerDelete:
		handlers.OnContainerDelete(t)
	default:
		log.Printf("No handler exists for event %s", t)
	}
}

func NewRuntimeInformer(client *containerd.Client) *RuntimeInformer {
	return &RuntimeInformer{
		client: client,
	}
}
