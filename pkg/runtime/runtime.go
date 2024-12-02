package runtime

import (
	"context"

	"github.com/amimof/blipblop/api/services/containers/v1"
	"github.com/amimof/blipblop/pkg/labels"
)

const (
	DefaultNamespace = "blipblop"
)

type Runtime interface {
	List(context.Context) ([]*containers.Container, error)
	Get(context.Context, string) (*containers.Container, error)
	Delete(context.Context, *containers.Container) error
	Kill(context.Context, *containers.Container) error
	Stop(context.Context, *containers.Container) error
	Run(context.Context, *containers.Container) error
	Cleanup(context.Context, *containers.Container) error
	Pull(context.Context, *containers.Container) error
	Labels(context.Context) (labels.Label, error)
	Namespace() string
}
