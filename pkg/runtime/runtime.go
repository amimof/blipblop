// Package runtime provides an interface for container runtime environments as well as some builtin implementations
// such as containerd.
package runtime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"

	"github.com/amimof/blipblop/api/services/containers/v1"
	"github.com/amimof/blipblop/pkg/labels"
)

const (
	DefaultNamespace = "blipblop"
	IDMaxLen         = 64
)

type ID string

func (id ID) String() string {
	return string(id)
}

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
	Labels(context.Context, string) (labels.Label, error)
	IO(context.Context, string) (*ContainerIO, error)
	Namespace() string
	Version(context.Context) (string, error)
	ID(context.Context, string) (string, error)
	Name(context.Context, string) (string, error)
}

func GenerateID() ID {
	blen := IDMaxLen / 2
	b := make([]byte, blen)
	_, _ = rand.Read(b)
	return ID(hex.EncodeToString(b))
}
