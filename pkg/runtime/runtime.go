// Package runtime provides an interface for container runtime environments as well as some builtin implementations
// such as containerd.
package runtime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"

	"github.com/amimof/voiyd/pkg/labels"

	taskv1 "github.com/amimof/voiyd/api/services/tasks/v1"
)

const (
	DefaultNamespace = "voiyd"
	IDMaxLen         = 64
)

type ID string

func (id ID) String() string {
	return string(id)
}

type TaskIO struct {
	Stdout io.ReadCloser
	Stderr io.ReadCloser
}

type Runtime interface {
	List(context.Context) ([]*taskv1.Task, error)
	Get(context.Context, string) (*taskv1.Task, error)
	Delete(context.Context, *taskv1.Task) error
	Kill(context.Context, *taskv1.Task) error
	Stop(context.Context, *taskv1.Task) error
	Run(context.Context, *taskv1.Task) error
	Cleanup(context.Context, string) error
	Pull(context.Context, *taskv1.Task) error
	Labels(context.Context, string) (labels.Label, error)
	IO(context.Context, string) (*TaskIO, error)
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
