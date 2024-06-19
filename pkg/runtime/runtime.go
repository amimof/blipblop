package runtime

import (
	"context"

	"github.com/amimof/blipblop/api/services/containers/v1"
)

type Runtime interface {
	List(context.Context) ([]*containers.Container, error)
	Get(context.Context, string) (*containers.Container, error)
	Create(context.Context, *containers.Container) error
	Delete(context.Context, string) error
	Kill(context.Context, string) error
	Stop(context.Context, string) error
	Start(context.Context, string) error
	IsServing(context.Context) (bool, error)
}
