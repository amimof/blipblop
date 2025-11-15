// Package scheduling provides interface to implement workload schedulers
package scheduling

import (
	"context"
	"errors"

	"github.com/amimof/blipblop/api/services/containers/v1"
	"github.com/amimof/blipblop/api/services/nodes/v1"
)

var ErrSchedulingNoNode = errors.New("no node fit for scheduling")

type Scheduler interface {
	Score(context.Context, *containers.Container) (map[string]float64, error)
	Schedule(context.Context, *containers.Container) (*nodes.Node, error)
}
