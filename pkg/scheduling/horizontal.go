package scheduling

import (
	"context"
	"math/rand"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/amimof/voiyd/pkg/client"
	"github.com/amimof/voiyd/pkg/condition"
	"github.com/amimof/voiyd/pkg/labels"
	"github.com/amimof/voiyd/pkg/util"

	nodesv1 "github.com/amimof/voiyd/api/services/nodes/v1"
	tasksv1 "github.com/amimof/voiyd/api/services/tasks/v1"
)

type horizontal struct {
	clientset *client.ClientSet
}

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

func (s *horizontal) Score(ctx context.Context, c *tasksv1.Task) (map[string]float64, error) {
	return nil, nil
}

func (s *horizontal) Schedule(ctx context.Context, c *tasksv1.Task) (*nodesv1.Node, error) {
	// List all nodes
	allNodes, err := s.clientset.NodeV1().List(ctx)
	if err != nil {
		return nil, err
	}

	// Don't attempt to schedule on a Unready node
	filteredNodes := filterByState(allNodes, string(condition.ReasonReady))

	// Make sure we have at least 1 node in the cluster
	if len(filteredNodes) < 1 {
		return nil, ErrSchedulingNoNode
	}

	// Filter nodes depending on nodeSelector
	filteredNodes = filterByNodeSelector(filteredNodes, c.GetConfig().GetNodeSelector())

	// Check if the container belongs to a set
	setLabelKey := labels.LabelPrefix("container-set").String()
	if setName, ok := c.GetMeta().GetLabels()[setLabelKey]; ok {

		filter := labels.New()
		filter.Set(setLabelKey, setName)

		// Get all containers in set
		tasks, err := s.clientset.TaskV1().List(ctx, filter)
		if err != nil {
			return nil, err
		}

		// Remove nodes that already have containers from the same set

		containersInSet := filteredNodes
		for _, task := range tasks {
			containersInSet = excludeByName(containersInSet, task.GetStatus().GetNode().GetValue())
		}

		// If no nodes are available, then schedule on a node that has containers from same set
		if len(containersInSet) > 0 {
			filteredNodes = containersInSet
		}

	}

	// Choose a node by random out of the filtered list of nodes
	return pickRandomNode(filteredNodes)
}

func NewHorizontalScheduler(c *client.ClientSet) Scheduler {
	return &horizontal{
		clientset: c,
	}
}
