package scheduling

import (
	"context"
	"testing"

	"go.uber.org/mock/gomock"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/amimof/voiyd/api/types/v1"
	"github.com/amimof/voiyd/pkg/client"
	"github.com/amimof/voiyd/pkg/condition"
	"github.com/amimof/voiyd/pkg/labels"
	"github.com/stretchr/testify/assert"

	nodesv1 "github.com/amimof/voiyd/api/services/nodes/v1"
	tasksv1 "github.com/amimof/voiyd/api/services/tasks/v1"
	nodes "github.com/amimof/voiyd/pkg/client/node/v1"
	tasks "github.com/amimof/voiyd/pkg/client/task/v1"
)

type want struct {
	expect any
	call   *gomock.Call
}

var testNodes = &nodesv1.ListResponse{
	Nodes: []*nodesv1.Node{
		{
			Meta: &types.Meta{
				Name: "node-a",
				Labels: map[string]string{
					labels.LabelPrefix("plattform").String(): "linux/amd64",
				},
			},
			Status: &nodesv1.Status{
				Phase: wrapperspb.String(string(condition.ReasonConnected)),
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
				Phase: wrapperspb.String(string(condition.ReasonConnected)),
			},
		},
		{
			Meta: &types.Meta{
				Name: "node-d",
			},
			Status: &nodesv1.Status{
				Phase: wrapperspb.String(string(condition.ReasonConnected)),
			},
		},
	},
}

var testTasks = []*tasksv1.Task{
	{
		Meta: &types.Meta{
			Name: "container-a",
		},
		Config: &tasksv1.Config{
			Image: "docker.io/library/nginx:latest",
		},
	},
	{
		Meta: &types.Meta{
			Name: "container-b",
		},
		Config: &tasksv1.Config{
			Image: "docker.io/library/nginx:latest",
		},
	},
	{
		Meta: &types.Meta{
			Name: "container-c",
		},
		Config: &tasksv1.Config{
			Image: "docker.io/library/nginx:latest",
		},
	},

	// Tasks part of a 2-replica set: Set A
	{
		Meta: &types.Meta{
			Name: "set-a-container-a",
			Labels: map[string]string{
				labels.LabelPrefix("container-set").String(): "set-a",
			},
		},
		Config: &tasksv1.Config{
			Image: "docker.io/library/redis:latest",
		},
		Status: &tasksv1.Status{
			Node: wrapperspb.String("node-a"),
		},
	},
	{
		Meta: &types.Meta{
			Name: "set-a-container-b",
			Labels: map[string]string{
				labels.LabelPrefix("container-set").String(): "set-a",
			},
		},
		Config: &tasksv1.Config{
			Image: "docker.io/library/redis:latest",
		},
		Status: &tasksv1.Status{
			Node: wrapperspb.String("node-b"),
		},
	},
	{
		Meta: &types.Meta{
			Name: "set-a-container-b",
			Labels: map[string]string{
				labels.LabelPrefix("container-set").String(): "set-a",
			},
		},
		Config: &tasksv1.Config{
			Image: "docker.io/library/redis:latest",
		},
		Status: &tasksv1.Status{
			Node: wrapperspb.String("node-c"),
		},
	},

	// Tasks part of a 2-replica set: Set B
	{
		Meta: &types.Meta{
			Name: "set-b-container-a",
			Labels: map[string]string{
				labels.LabelPrefix("container-set").String(): "set-b",
			},
		},
		Config: &tasksv1.Config{
			Image: "docker.io/library/redis:latest",
		},
		Status: &tasksv1.Status{
			Node: wrapperspb.String("node-a"),
		},
	},
	{
		Meta: &types.Meta{
			Name: "set-b-container-b",
			Labels: map[string]string{
				labels.LabelPrefix("container-set").String(): "set-b",
			},
		},
		Config: &tasksv1.Config{
			Image: "docker.io/library/redis:latest",
		},
		Status: &tasksv1.Status{
			Node: wrapperspb.String("node-b"),
		},
	},
}

func TestHoriontalSchedulerFilters(t *testing.T) {
	in := testNodes.GetNodes()

	in = filterByState(in, string(condition.ReasonConnected))
	assert.Len(t, in, 3, "Number of nodes should be 3")
	for _, n := range in {
		assert.NotEqual(t, n.GetMeta().GetName(), "node-b", "Unready nodes should should not exist in result")
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
	containerMockClient := tasks.NewMockTaskServiceClient(ctrl)
	nodeMockClient := nodes.NewMockNodeServiceClient(ctrl)
	clientV1 := tasks.NewClientV1(tasks.WithClient(containerMockClient))
	nodesV1 := nodes.NewClientV1(nodes.WithClient(nodeMockClient))

	// Setup Mocks
	mocks := map[string]want{
		"ListNodes": {
			call:   nodeMockClient.EXPECT().List(gomock.Any(), gomock.Any()).AnyTimes(),
			expect: testNodes,
		},
		"ListTasks": {
			call: containerMockClient.EXPECT().List(gomock.Any(), gomock.Any()).AnyTimes(),
			expect: &tasksv1.ListResponse{
				Tasks: testTasks,
			},
		},
	}

	// Run node mocks
	for _, tc := range mocks {
		tc.call.Return(tc.expect, nil)
	}

	// Setup Scheduler
	cs := &client.ClientSet{TaskV1Client: clientV1, NodeV1Client: nodesV1}
	scheduler := NewHorizontalScheduler(cs)

	cases := map[string]struct {
		input       *tasksv1.Task
		expectOneOf []*nodesv1.Node
	}{
		"TaskInSetA": {
			input: &tasksv1.Task{
				Meta: &types.Meta{
					Name: "amirs-container",
					Labels: map[string]string{
						labels.LabelPrefix("container-set").String(): "set-a",
					},
				},
				Config: &tasksv1.Config{
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
					Status: &nodesv1.Status{
						Phase: wrapperspb.String(string(condition.ReasonConnected)),
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
						Phase: wrapperspb.String(string(condition.ReasonConnected)),
					},
				},
			},
		},
		"TaskWithNodeSelector": {
			input: &tasksv1.Task{
				Meta: &types.Meta{
					Name: "amirs-container",
					Labels: map[string]string{
						labels.LabelPrefix("container-set").String(): "set-a",
					},
				},
				Config: &tasksv1.Config{
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
					Status: &nodesv1.Status{
						Phase: wrapperspb.String(string(condition.ReasonConnected)),
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
