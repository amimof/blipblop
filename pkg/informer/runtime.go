package informer

import (
	//"os"
	"context"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/api/events"
	"github.com/containerd/typeurl"
	"github.com/sirupsen/logrus"
)

type ContainerdEvent interface {
	ProtoMessage()
}

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
				logrus.Printf("Error: %s", err.Error())
			}
			handleEvent(i.handlers, ev)
		case err := <-errs:
			if err != nil {
				logrus.Printf("Error %s", err)
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
		if handlers.OnTaskExit != nil {
			handlers.OnTaskExit(t)
		}
	case *events.TaskCreate:
		if handlers.OnTaskCreate != nil {
			handlers.OnTaskCreate(t)
		}
	case *events.TaskStart:
		if handlers.OnTaskStart != nil {
			handlers.OnTaskStart(t)
		}
	case *events.TaskDelete:
		if handlers.OnTaskDelete != nil {
			handlers.OnTaskDelete(t)
		}
	case *events.TaskIO:
		if handlers.OnTaskIO != nil {
			handlers.OnTaskIO(t)
		}
	case *events.TaskOOM:
		if handlers.OnTaskOOM != nil {
			handlers.OnTaskOOM(t)
		}
	case *events.TaskExecAdded:
		if handlers.OnTaskExecAdded != nil {
			handlers.OnTaskExecAdded(t)
		}
	case *events.TaskExecStarted:
		if handlers.OnTaskExecStarted != nil {
			handlers.OnTaskExecStarted(t)
		}
	case *events.TaskPaused:
		if handlers.OnTaskPaused != nil {
			handlers.OnTaskPaused(t)
		}
	case *events.TaskResumed:
		if handlers.OnTaskResumed != nil {
			handlers.OnTaskResumed(t)
		}
	case *events.TaskCheckpointed:
		if handlers.OnTaskCheckpointed != nil {
			handlers.OnTaskCheckpointed(t)
		}
	case *events.SnapshotPrepare:
		if handlers.OnSnapshotPrepare != nil {
			handlers.OnSnapshotPrepare(t)
		}
	case *events.SnapshotCommit:
		if handlers.OnSnapshotCommit != nil {
			handlers.OnSnapshotCommit(t)
		}
	case *events.SnapshotRemove:
		if handlers.OnSnapshotRemove != nil {
			handlers.OnSnapshotRemove(t)
		}
	case *events.NamespaceCreate:
		if handlers.OnNamespaceCreate != nil {
			handlers.OnNamespaceCreate(t)
		}
	case *events.NamespaceUpdate:
		if handlers.OnNamespaceUpdate != nil {
			handlers.OnNamespaceUpdate(t)
		}
	case *events.NamespaceDelete:
		if handlers.OnNamespaceDelete != nil {
			handlers.OnNamespaceDelete(t)
		}
	case *events.ImageCreate:
		if handlers.OnImageCreate != nil {
			handlers.OnImageCreate(t)
		}
	case *events.ImageUpdate:
		if handlers.OnImageUpdate != nil {
			handlers.OnImageUpdate(t)
		}
	case *events.ImageDelete:
		if handlers.OnImageDelete != nil {
			handlers.OnImageDelete(t)
		}
	case *events.ContentDelete:
		if handlers.OnContentDelete != nil {
			handlers.OnContentDelete(t)
		}
	case *events.ContainerCreate:
		if handlers.OnContainerCreate != nil {
			handlers.OnContainerCreate(t)
		}
	case *events.ContainerUpdate:
		if handlers.OnContainerUpdate != nil {
			handlers.OnContainerUpdate(t)
		}
	case *events.ContainerDelete:
		if handlers.OnContainerDelete != nil {
			handlers.OnContainerDelete(t)
		}
	default:
		logrus.Printf("No handler exists for event %s", t)
	}
}

func NewRuntimeInformer(client *containerd.Client) *RuntimeInformer {
	return &RuntimeInformer{
		client: client,
	}
}
