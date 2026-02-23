package scheduling

import (
	"context"
	"math/rand"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/amimof/voiyd/pkg/labels"
	"github.com/amimof/voiyd/pkg/util"

	nodesv1 "github.com/amimof/voiyd/api/services/nodes/v1"
	tasksv1 "github.com/amimof/voiyd/api/services/tasks/v1"
)

type horizontal struct{}

func excludeByName(original []*nodesv1.Node, nodeName string) []*nodesv1.Node {
	var result []*nodesv1.Node
	copied := util.CopyList(original)
	for _, node := range copied {
		if nodeName != node.GetMeta().GetName() {
			newItem := proto.Clone(node).(*nodesv1.Node)
			result = append(result, newItem)
		}
	}
	return result
}

func filterByNodeSelector(original []*nodesv1.Node, l labels.Label) []*nodesv1.Node {
	var result []*nodesv1.Node
	copied := util.CopyList(original)
	for _, node := range copied {
		filter := labels.NewCompositeSelectorFromMap(l)
		if filter.Matches(node.GetMeta().GetLabels()) {
			newItem := proto.Clone(node).(*nodesv1.Node)
			result = append(result, newItem)
		}
	}
	return result
}

func filterByState(original []*nodesv1.Node, state string) []*nodesv1.Node {
	var result []*nodesv1.Node
	for _, node := range original {
		if state == node.GetStatus().GetPhase().GetValue() {
			result = append(result, node)
		}
	}
	return result
}

func pickRandomNode(original []*nodesv1.Node) (*nodesv1.Node, error) {
	if len(original) <= 0 {
		return nil, ErrSchedulingNoNode
	}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	i := r.Intn(len(original))
	return original[i], nil
}

// Checks if there are nodes that matches the task's nodeSelector.
// Returns true if at least one node has matching labels.
// Returns false if no nodes has matching labels.
func hasMatchingNodes(task *tasksv1.Task, nodes []*nodesv1.Node) (bool, error) {
	// Check if any node matches the task's nodeSelector
	selector := labels.NewCompositeSelectorFromMap(task.GetConfig().GetNodeSelector())
	for _, node := range nodes {
		if selector.Matches(node.GetMeta().GetLabels()) {
			return true, nil
		}
	}

	return false, nil
}

func (s *horizontal) Score(_ context.Context, _ *tasksv1.Task, _ []*nodesv1.Node) (map[string]float64, error) {
	return nil, nil
}

// TODO: Fix scheduling algorithm so Disconnected nodes are excluded
func (s *horizontal) Schedule(ctx context.Context, t *tasksv1.Task, allNodes []*nodesv1.Node) (*nodesv1.Node, error) {
	// Don't attempt to schedule on a Unready node
	// filteredNodes := filterByState(allNodes, string(condition.ReasonReady))
	filteredNodes := allNodes

	match, err := hasMatchingNodes(t, filteredNodes)
	if err != nil {
		return nil, err
	}

	if !match {
		return nil, ErrSchedulingNoMatchingNode
	}
	// Make sure we have at least 1 node in the cluster
	if len(filteredNodes) < 1 {
		return nil, ErrSchedulingNoNode
	}

	// Filter nodes depending on nodeSelector
	filteredNodes = filterByNodeSelector(filteredNodes, t.GetConfig().GetNodeSelector())

	// Choose a node by random out of the filtered list of nodes
	return pickRandomNode(filteredNodes)
}

func NewHorizontalScheduler() Scheduler {
	return &horizontal{}
}
