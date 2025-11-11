package scheduling

import (
	"context"
	"errors"
	"math/rand"
	"time"

	"github.com/amimof/blipblop/api/services/containers/v1"
	"github.com/amimof/blipblop/api/services/nodes/v1"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/amimof/blipblop/pkg/labels"
	"github.com/amimof/blipblop/pkg/node"
	"github.com/amimof/blipblop/pkg/util"
	"google.golang.org/protobuf/proto"
)

var ErrSchedulingNoNode = errors.New("no node fit for scheduling")

type Scheduler interface {
	Score(context.Context, *containers.Container) (map[string]float64, error)
	Schedule(context.Context, *containers.Container) (*nodes.Node, error)
}

type horizontal struct {
	clientset *client.ClientSet
}

func excludeByName(original []*nodes.Node, nodeName string) []*nodes.Node {
	var result []*nodes.Node
	copied := util.CopyList(original)
	for _, node := range copied {
		if nodeName != node.GetMeta().GetName() {
			newItem := proto.Clone(node).(*nodes.Node)
			result = append(result, newItem)
		}
	}
	return result
}

func filterByNodeSelector(original []*nodes.Node, l labels.Label) []*nodes.Node {
	var result []*nodes.Node
	copied := util.CopyList(original)
	for _, node := range copied {
		filter := labels.NewCompositeSelectorFromMap(l)
		if filter.Matches(node.GetMeta().GetLabels()) {
			newItem := proto.Clone(node).(*nodes.Node)
			result = append(result, newItem)
		}
	}
	return result
}

func excludeByState(original []*nodes.Node, state string) []*nodes.Node {
	var result []*nodes.Node
	for _, node := range original {
		if state != node.GetStatus().GetPhase().String() {
			newItem := proto.Clone(node).(*nodes.Node)
			result = append(result, newItem)
		}
	}
	return result
}

func pickRandomNode(original []*nodes.Node) *nodes.Node {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	// Schedule on random node as a last resort
	i := r.Intn(len(original))
	return original[i]
}

// func filterByContainerSet(original []*nodes.Node, cs *v1.ClientV1, setName string) *nodes.Node {
//
// 	setLabelKey := labels.LabelPrefix("container-set").String()
// 	if setName, ok := c.GetMeta().GetLabels()[setLabelKey]; ok {
//
// 		filter := labels.New()
// 		filter.Set(setLabelKey, setName)
//
// 		// Get all containers in set
// 		ctrs, err := s.clientset.ContainerV1().List(ctx, filter)
// 		if err != nil {
// 			return nil, err
// 		}
//
// 		// Remove nodes that already have containers from the same set
// 		for _, ctr := range ctrs {
// 			filteredNodes = excludeByName(filteredNodes, ctr.GetStatus().GetNode())
// 		}
//   }
// }

func (s *horizontal) Score(ctx context.Context, c *containers.Container) (map[string]float64, error) {
	return nil, nil
}

func (s *horizontal) Schedule(ctx context.Context, c *containers.Container) (*nodes.Node, error) {
	// List all nodes
	allNodes, err := s.clientset.NodeV1().List(ctx)
	if err != nil {
		return nil, err
	}

	// Don't attempt to schedule on a Unready node
	filteredNodes := excludeByState(allNodes, node.StatusMissing)

	// Make sure we have at least 1 node in the cluster
	if len(allNodes) < 1 {
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
		ctrs, err := s.clientset.ContainerV1().List(ctx, filter)
		if err != nil {
			return nil, err
		}

		// Remove nodes that already have containers from the same set
		var containersInSet []*nodes.Node
		for _, ctr := range ctrs {
			containersInSet = excludeByName(filteredNodes, ctr.GetStatus().GetNode().String())
		}

		// If no nodes are available, then schedule on a node that has containers from same set
		if len(containersInSet) > 0 {
			filteredNodes = containersInSet
		}

	}

	var res *nodes.Node

	// Schedule on random node as a last resort
	if len(filteredNodes) < 1 {
		n := pickRandomNode(allNodes)
		filteredNodes = append(filteredNodes, n)
	}

	// Choose a node by random out of the filtered list of nodes
	res = pickRandomNode(filteredNodes)

	return res, nil
}

func NewHorizontalScheduler(c *client.ClientSet) Scheduler {
	return &horizontal{
		clientset: c,
	}
}
