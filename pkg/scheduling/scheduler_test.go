package scheduling

import (
	"context"
	"testing"

	containersv1 "github.com/amimof/blipblop/api/services/containers/v1"
	nodesv1 "github.com/amimof/blipblop/api/services/nodes/v1"
	"github.com/amimof/blipblop/api/types/v1"
	"github.com/amimof/blipblop/pkg/client"
	containers "github.com/amimof/blipblop/pkg/client/container/v1"
	nodes "github.com/amimof/blipblop/pkg/client/node/v1"
	"github.com/amimof/blipblop/pkg/labels"
	"github.com/amimof/blipblop/pkg/node"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"google.golang.org/protobuf/proto"
)

type want struct {
	expect any
	call   *gomock.Call
}

var testNodes = &nodesv1.ListNodeResponse{
	Nodes: []*nodesv1.Node{
		{
			Meta: &types.Meta{
				Name: "node-a",
				Labels: map[string]string{
					labels.LabelPrefix("plattform").String(): "linux/amd64",
				},
			},
		},
		{
			Meta: &types.Meta{
				Name: "node-b",
			},
		},
		{
			Meta: &types.Meta{
				Name: "node-c",
			},
			Status: &nodesv1.Status{
				State: node.StatusReady,
			},
		},
		{
			Meta: &types.Meta{
				Name: "node-d",
			},
			Status: &nodesv1.Status{
				State: node.StatusMissing,
			},
		},
	},
}

var testContainers = []*containersv1.Container{
	{
		Meta: &types.Meta{
			Name: "container-a",
		},
		Config: &containersv1.Config{
			Image: "docker.io/library/nginx:latest",
		},
	},
	{
		Meta: &types.Meta{
			Name: "container-b",
		},
		Config: &containersv1.Config{
			Image: "docker.io/library/nginx:latest",
		},
	},
	{
		Meta: &types.Meta{
			Name: "container-c",
		},
		Config: &containersv1.Config{
			Image: "docker.io/library/nginx:latest",
		},
	},

	// Containers part of a 2-replica set: Set A
	{
		Meta: &types.Meta{
			Name: "set-a-container-a",
			Labels: map[string]string{
				labels.LabelPrefix("container-set").String(): "set-a",
			},
		},
		Config: &containersv1.Config{
			Image: "docker.io/library/redis:latest",
		},
		Status: &containersv1.Status{
			Node: "node-a",
		},
	},
	{
		Meta: &types.Meta{
			Name: "set-a-container-b",
			Labels: map[string]string{
				labels.LabelPrefix("container-set").String(): "set-a",
			},
		},
		Config: &containersv1.Config{
			Image: "docker.io/library/redis:latest",
		},
		Status: &containersv1.Status{
			Node: "node-b",
		},
	},
	{
		Meta: &types.Meta{
			Name: "set-a-container-b",
			Labels: map[string]string{
				labels.LabelPrefix("container-set").String(): "set-a",
			},
		},
		Config: &containersv1.Config{
			Image: "docker.io/library/redis:latest",
		},
		Status: &containersv1.Status{
			Node: "node-c",
		},
	},

	// Containers part of a 2-replica set: Set B
	{
		Meta: &types.Meta{
			Name: "set-b-container-a",
			Labels: map[string]string{
				labels.LabelPrefix("container-set").String(): "set-b",
			},
		},
		Config: &containersv1.Config{
			Image: "docker.io/library/redis:latest",
		},
		Status: &containersv1.Status{
			Node: "node-a",
		},
	},
	{
		Meta: &types.Meta{
			Name: "set-b-container-b",
			Labels: map[string]string{
				labels.LabelPrefix("container-set").String(): "set-b",
			},
		},
		Config: &containersv1.Config{
			Image: "docker.io/library/redis:latest",
		},
		Status: &containersv1.Status{
			Node: "node-b",
		},
	},
}

func TestHoriontalSchedulerFilters(t *testing.T) {
	in := testNodes.GetNodes()

	in = excludeByState(in, node.StatusMissing)
	assert.Len(t, in, 3, "Number of nodes should be 3")
	for _, n := range in {
		assert.NotEqual(t, n.GetMeta().GetName(), "node-d", "Nodes with MISSING status should not exist in result")
	}

	in = testNodes.GetNodes()
	in = excludeByName(in, "node-a")
	assert.Len(t, in, 3, "Number of nodes should be 3")
	for _, n := range in {
		assert.NotEqual(t, n.GetMeta().GetName(), "node-a", "Node by the name node-a should not exist in result")
	}

	filter := labels.New()
	filter.Set(labels.LabelPrefix("plattform").String(), "linux/amd64")
	in = testNodes.GetNodes()
	in = filterByNodeSelector(in, filter)

	assert.Len(t, in, 1, "Number of nodes should be 1")
	assert.Equal(t, in[0].GetMeta().GetName(), "node-a")
}

func TestHorizontalSchedulerSingleNode(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mock client
	containerMockClient := containers.NewMockContainerServiceClient(ctrl)
	nodeMockClient := nodes.NewMockNodeServiceClient(ctrl)
	// clientV1 := containers.ClientV1{Client: containerMockClient}
	clientV1 := containers.NewClientV1(containers.WithClient(containerMockClient))
	nodesV1 := nodes.ClientV1{Client: nodeMockClient}

	// Setup Mocks
	mocks := map[string]want{
		"ListNodes": {
			call:   nodeMockClient.EXPECT().List(gomock.Any(), gomock.Any()).AnyTimes(),
			expect: testNodes,
		},
		"ListContainers": {
			call: containerMockClient.EXPECT().List(gomock.Any(), gomock.Any()).AnyTimes(),
			expect: &containersv1.ListContainerResponse{
				Containers: testContainers,
			},
		},
	}

	// Run node mocks
	for _, tc := range mocks {
		tc.call.Return(tc.expect, nil)
	}

	// Setup Scheduler
	cs := &client.ClientSet{ContainerV1Client: clientV1, NodeV1Client: &nodesV1}
	scheduler := NewHorizontalScheduler(cs)

	cases := map[string]struct {
		input       *containersv1.Container
		expectOneOf []*nodesv1.Node
	}{
		"ContainerInSetA": {
			input: &containersv1.Container{
				Meta: &types.Meta{
					Name: "amirs-container",
					Labels: map[string]string{
						labels.LabelPrefix("container-set").String(): "set-a",
					},
				},
				Config: &containersv1.Config{
					Image: "docker.io/library/redis:latest",
				},
			},
			expectOneOf: []*nodesv1.Node{
				{
					Meta: &types.Meta{
						Name: "node-a",
						Labels: map[string]string{
							labels.LabelPrefix("plattform").String(): "linux/amd64",
						},
					},
				},
				{
					Meta: &types.Meta{
						Name: "node-b",
					},
				},
				{
					Meta: &types.Meta{
						Name: "node-c",
					},
					Status: &nodesv1.Status{
						State: node.StatusReady,
					},
				},
			},
		},
		"ContainerWithNodeSelector": {
			input: &containersv1.Container{
				Meta: &types.Meta{
					Name: "amirs-container",
					Labels: map[string]string{
						labels.LabelPrefix("container-set").String(): "set-a",
					},
				},
				Config: &containersv1.Config{
					Image: "docker.io/library/redis:latest",
					NodeSelector: map[string]string{
						labels.LabelPrefix("plattform").String(): "linux/amd64",
					},
				},
			},
			expectOneOf: []*nodesv1.Node{
				{
					Meta: &types.Meta{
						Name: "node-a",
						Labels: map[string]string{
							labels.LabelPrefix("plattform").String(): "linux/amd64",
						},
					},
				},
			},
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			n, err := scheduler.Schedule(ctx, c.input)
			assert.NoError(t, err)

			var match bool
			for _, e := range c.expectOneOf {
				if proto.Equal(n, e) {
					match = true
					break
				}
			}
			assert.Truef(t, match, "Expect resulting node to match one of %v", c.expectOneOf)
		})
	}
}
