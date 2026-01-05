// Package scheduling provides interface to implement workload schedulers
package scheduling

import (
	"context"
	"errors"

	nodesv1 "github.com/amimof/voiyd/api/services/nodes/v1"
	tasksv1 "github.com/amimof/voiyd/api/services/tasks/v1"
)

var ErrSchedulingNoNode = errors.New("no node fit for scheduling")

type Scheduler interface {
	Score(context.Context, *tasksv1.Task) (map[string]float64, error)
	Schedule(context.Context, *tasksv1.Task) (*nodesv1.Node, error)
}
