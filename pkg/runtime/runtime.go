// Package runtime provides an interface for container runtime environments as well as some builtin implementations
// such as containerd.
package runtime

import (
	"context"
	"io"

	"github.com/amimof/blipblop/api/services/containers/v1"
	"github.com/amimof/blipblop/pkg/labels"
)

const (
	DefaultNamespace = "blipblop"
)

type ContainerIO struct {
	Stdout io.ReadCloser
	Stderr io.ReadCloser
}

type Runtime interface {
	List(context.Context) ([]*containers.Container, error)
	Get(context.Context, string) (*containers.Container, error)
	Delete(context.Context, *containers.Container) error
	Kill(context.Context, *containers.Container) error
	Stop(context.Context, *containers.Container) error
	Run(context.Context, *containers.Container) error
	Cleanup(context.Context, string) error
	Pull(context.Context, *containers.Container) error
	Labels(context.Context) (labels.Label, error)
	IO(context.Context, string) (*ContainerIO, error)
	Namespace() string
	Version(context.Context) (string, error)
}
