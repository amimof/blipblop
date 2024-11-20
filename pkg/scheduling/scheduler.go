package scheduling

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/amimof/blipblop/api/services/containers/v1"
	"github.com/amimof/blipblop/api/services/nodes/v1"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/amimof/blipblop/pkg/labels"
)

type Scheduler interface {
	Score(context.Context, *containers.Container) (map[string]float64, error)
	Schedule(context.Context, *containers.Container) (*nodes.Node, error)
}

type horizontal struct {
	clientset *client.ClientSet
	nodes     []*nodes.Node
}

func excludeNodeFrom(original []*nodes.Node, nodeName string) []*nodes.Node {
	var result []*nodes.Node
	for _, node := range original {
		if nodeName != node.GetMeta().GetName() {
			result = append(result, node)
		}
	}
	return result
}

func filterByNodeSelector(original []*nodes.Node, l labels.Label) []*nodes.Node {
	var result []*nodes.Node
	for _, node := range original {
		filter := labels.NewCompositeSelectorFromMap(l)
		if filter.Matches(node.GetMeta().GetLabels()) {
			result = append(result, node)
		}
	}
	return result
}

func (s *horizontal) Score(ctx context.Context, c *containers.Container) (map[string]float64, error) {
	return nil, nil
}

func (s *horizontal) Schedule(ctx context.Context, c *containers.Container) (*nodes.Node, error) {
	// List all nodes
	allNodes, err := s.clientset.NodeV1().List(ctx)
	if err != nil {
		return nil, err
	}

	// Make sure we have at least 1 node in the cluster
	if len(allNodes) < 1 {
		return nil, fmt.Errorf("no nodes to schedule on")
	}

	// Filter nodes depending on nodeSelector
	filteredNodes := filterByNodeSelector(allNodes, c.GetConfig().GetNodeSelector())

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
		for _, ctr := range ctrs {
			filteredNodes = excludeNodeFrom(filteredNodes, ctr.GetStatus().GetNode())
		}
	}

	var res *nodes.Node
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Schedule on random node as a last resort
	if len(filteredNodes) < 1 {
		i := r.Intn(len(allNodes))
		filteredNodes = append(filteredNodes, allNodes[i])
	}

	// Choose a node by random out of the filtered list of nodes
	i := r.Intn(len(filteredNodes))
	res = filteredNodes[i]

	return res, nil
}

func NewHorizontalScheduler(c *client.ClientSet) Scheduler {
	return &horizontal{
		clientset: c,
	}
}
