// Package repository provides interfaces for implementing storage solutions for types
package repository

import (
	"context"
	"errors"

	"github.com/amimof/voiyd/pkg/labels"

	containersetsv1 "github.com/amimof/voiyd/api/services/containersets/v1"
	eventsv1 "github.com/amimof/voiyd/api/services/events/v1"
	nodesv1 "github.com/amimof/voiyd/api/services/nodes/v1"
	tasksv1 "github.com/amimof/voiyd/api/services/tasks/v1"
	volumesv1 "github.com/amimof/voiyd/api/services/volumes/v1"
)

var ErrNotFound = errors.New("item not found")

type ContainerSetRepository interface {
	Create(context.Context, *containersetsv1.ContainerSet) error
	Get(context.Context, string) (*containersetsv1.ContainerSet, error)
	Delete(context.Context, string) error
	List(context.Context) ([]*containersetsv1.ContainerSet, error)
	Update(context.Context, *containersetsv1.ContainerSet) error
}

type TaskRepository interface {
	Create(context.Context, *tasksv1.Task) error
	Get(context.Context, string) (*tasksv1.Task, error)
	Delete(context.Context, string) error
	List(context.Context, ...labels.Label) ([]*tasksv1.Task, error)
	Update(context.Context, *tasksv1.Task) error
}

type NodeRepository interface {
	Create(context.Context, *nodesv1.Node) error
	Get(context.Context, string) (*nodesv1.Node, error)
	Delete(context.Context, string) error
	List(context.Context) ([]*nodesv1.Node, error)
	Update(context.Context, *nodesv1.Node) error
}

type EventRepository interface {
	Create(context.Context, *eventsv1.Event) error
	Get(context.Context, string) (*eventsv1.Event, error)
	Delete(context.Context, string) error
	List(context.Context) ([]*eventsv1.Event, error)
}

type VolumeRepository interface {
	Create(context.Context, *volumesv1.Volume) error
	Get(context.Context, string) (*volumesv1.Volume, error)
	Delete(context.Context, string) error
	List(context.Context, ...labels.Label) ([]*volumesv1.Volume, error)
	Update(context.Context, *volumesv1.Volume) error
}
