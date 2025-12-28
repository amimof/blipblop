// Package repository provides interfaces for implementing storage solutions for types
package repository

import (
	"context"
	"errors"

	containersv1 "github.com/amimof/blipblop/api/services/containers/v1"
	containersetsv1 "github.com/amimof/blipblop/api/services/containersets/v1"
	eventsv1 "github.com/amimof/blipblop/api/services/events/v1"
	nodesv1 "github.com/amimof/blipblop/api/services/nodes/v1"
	volumesv1 "github.com/amimof/blipblop/api/services/volumes/v1"
	"github.com/amimof/blipblop/pkg/labels"
)

var ErrNotFound = errors.New("item not found")

type ContainerSetRepository interface {
	Create(context.Context, *containersetsv1.ContainerSet) error
	Get(context.Context, string) (*containersetsv1.ContainerSet, error)
	Delete(context.Context, string) error
	List(context.Context) ([]*containersetsv1.ContainerSet, error)
	Update(context.Context, *containersetsv1.ContainerSet) error
}

type ContainerRepository interface {
	Create(context.Context, *containersv1.Container) error
	Get(context.Context, string) (*containersv1.Container, error)
	Delete(context.Context, string) error
	List(context.Context, ...labels.Label) ([]*containersv1.Container, error)
	Update(context.Context, *containersv1.Container) error
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
