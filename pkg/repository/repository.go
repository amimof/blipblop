package repository

import (
	"context"
	"errors"

	"github.com/amimof/blipblop/api/services/containers/v1"
	"github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/api/services/nodes/v1"
)

type ContainerRepository interface {
	Create(context.Context, *containers.Container) error
	Get(context.Context, string) (*containers.Container, error)
	Delete(context.Context, string) error
	List(context.Context) ([]*containers.Container, error)
	Update(context.Context, *containers.Container) error
}

type NodeRepository interface {
	Create(context.Context, *nodes.Node) error
	Get(context.Context, string) (*nodes.Node, error)
	Delete(context.Context, string) error
	List(context.Context) ([]*nodes.Node, error)
	Update(context.Context, *nodes.Node) error
}

type EventRepository interface {
	Create(context.Context, *events.Event) error
	Get(context.Context, string) (*events.Event, error)
	Delete(context.Context, string) error
	List(context.Context) ([]*events.Event, error)
	Update(context.Context, *events.Event) error
}

var ErrNotFound = errors.New("item not found")
