package protoutils

import (
	"encoding/json"
	"slices"
	"testing"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/amimof/voiyd/api/types/v1"

	tasksv1 "github.com/amimof/voiyd/api/services/tasks/v1"
)

func TestImmutableFields(t *testing.T) {
	existingTask := &tasksv1.Task{
		Meta: &types.Meta{
			Name: "test-task",
		},
		Config: &tasksv1.Config{
			Image: "test-image-new",
			PortMappings: []*tasksv1.PortMapping{
				{
					HostPort:   8080,
					TargetPort: 8080,
					Protocol:   "TCP",
				},
			},
		},
		Status: &tasksv1.Status{
			Phase: wrapperspb.String("idle"),
			Node:  wrapperspb.String("voiydnode"),
		},
	}

	patchTask := &tasksv1.Task{
		Config: &tasksv1.Config{
			Image: "test-image-new",
			PortMappings: []*tasksv1.PortMapping{
				{
					HostPort:   443,
					TargetPort: 8080,
					Protocol:   "TCP",
				},
				{
					HostPort:   8080,
					TargetPort: 8080,
					Protocol:   "TCP",
				},
			},
		},
	}

	updated, err := StrategicMergePatch(existingTask, patchTask)
	if err != nil {
		t.Error(err)
	}

	t.Logf("Result: %+v", updated)
}

func StrategicMergePatch(target, patch *tasksv1.Task) (*tasksv1.Task, error) {
	targetb, err := json.Marshal(target)
	if err != nil {
		return nil, err
	}

	patchb, err := json.Marshal(patch)
	if err != nil {
		return nil, err
	}

	b, err := jsonpatch.MergePatch(targetb, patchb)
	if err != nil {
		return nil, err
	}

	var c tasksv1.Task
	err = json.Unmarshal(b, &c)
	if err != nil {
		return nil, err
	}

	return &c, nil
}

func TestMergeNilSlices(t *testing.T) {
	type envVars struct {
		Name  string
		Value string
	}

	// Define the patch slice of Port items.
	patchPorts := []envVars{
		{Name: "https", Value: "8443"},
		{Name: "admin", Value: "8080"},
		{Name: "http", Value: "8080"},
	}

	// Simulate nil basePorts
	merged := MergeSlices(nil, patchPorts,
		func(item envVars) string {
			return item.Name
		},
		func(base, patch envVars) envVars {
			if patch.Value != "" {
				base.Value = patch.Value
			}
			return base
		},
	)

	expect := []envVars{
		{Name: "https", Value: "8443"},
		{Name: "admin", Value: "8080"},
		{Name: "http", Value: "8080"},
	}

	assert.Equal(t, expect, merged)
}

func TestMergeSlices(t *testing.T) {
	type envVars struct {
		Name  string
		Value string
	}

	// Define the base slice of Port items.
	basePorts := []envVars{
		{Name: "http", Value: "80"},
		{Name: "https", Value: "443"},
	}

	// Define the patch slice of Port items.
	patchPorts := []envVars{
		{Name: "https", Value: "8443"},
		{Name: "admin", Value: "8080"},
		{Name: "http", Value: "8080"},
	}

	merged := MergeSlices(basePorts, patchPorts,
		func(item envVars) string {
			return item.Name
		},
		func(base, patch envVars) envVars {
			if patch.Value != "" {
				base.Value = patch.Value
			}
			return base
		},
	)

	expect := []envVars{
		{Name: "http", Value: "8080"},
		{Name: "https", Value: "8443"},
		{Name: "admin", Value: "8080"},
	}

	assert.Equal(t, expect, merged)
}

func TestClearRepeatedFields(t *testing.T) {
	baseTask := tasksv1.Task{
		Meta: &types.Meta{
			Name: "test-task",
			Labels: map[string]string{
				"team":        "backend",
				"environment": "production",
				"role":        "root",
			},
		},
		Config: &tasksv1.Config{
			Image: "docker.io/library/nginx:latest",
			PortMappings: []*tasksv1.PortMapping{
				{
					Name:       "http",
					HostPort:   8080,
					TargetPort: 80,
					Protocol:   "TCP",
				},
			},
			Envvars: []*tasksv1.EnvVar{
				{
					Name:  "HTTP_PROXY",
					Value: "proxy.foo.com",
				},
			},
			Args: []string{
				"--config /mnt/cfg/config.yaml",
			},
			Mounts: []*tasksv1.Mount{
				{
					Name:        "temp",
					Source:      "/tmp",
					Destination: "/mnt/tmp",
					Type:        "bind",
				},
			},
			NodeSelector: map[string]string{
				"voiyd.io/arch": "amd64",
				"voiyd.io/os":   "linux",
			},
		},
	}
	expect := tasksv1.Task{
		Meta: &types.Meta{
			Name: "test-task",
			Labels: map[string]string{
				"team":        "backend",
				"environment": "production",
				"role":        "root",
			},
		},
		Config: &tasksv1.Config{
			Image:        "docker.io/library/nginx:latest",
			PortMappings: []*tasksv1.PortMapping{},
			Envvars:      []*tasksv1.EnvVar{},
			Args:         []string{},
			Mounts:       []*tasksv1.Mount{},
			NodeSelector: map[string]string{
				"voiyd.io/arch": "amd64",
				"voiyd.io/os":   "linux",
			},
		},
	}

	ClearRepeatedFields(&baseTask)

	if !proto.Equal(&baseTask, &expect) {
		t.Errorf("\ngot:\n%v\nwant:\n%v", &baseTask, &expect)
	}
}

func TestClearProto(t *testing.T) {
	testCases := []struct {
		name    string
		message proto.Message
		expect  proto.Message
	}{
		{
			name:   "should reset all fields on message",
			expect: &tasksv1.Status{},
			message: &tasksv1.Status{
				Phase:  wrapperspb.String("running"),
				Node:   wrapperspb.String("localhost"),
				Pid:    wrapperspb.UInt32(17778),
				Reason: wrapperspb.String("exit code 0"),
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			ClearProto(tt.message.ProtoReflect())
			if !proto.Equal(tt.message, tt.expect) {
				t.Errorf("\ngot:\n%v\nwant:\n%v", tt.message, tt.expect)
			}
		})
	}
}

func TestToFields(t *testing.T) {
	testCases := []struct {
		name    string
		message proto.Message
		expect  []string
	}{
		{
			name:   "should have field paths for wrappers",
			expect: []string{"phase", "node", "pid", "reason"},
			message: &tasksv1.Status{
				Phase:  wrapperspb.String("running"),
				Node:   wrapperspb.String("localhost"),
				Pid:    wrapperspb.UInt32(17778),
				Reason: wrapperspb.String("exit code 0"),
			},
		},
		{
			name:   "should have field paths for scalars",
			expect: []string{"node", "ip"},
			message: &tasksv1.Status{
				Node: wrapperspb.String("node-01"),
				Ip:   wrapperspb.String("172.19.1.123"),
			},
		},
		{
			name:   "should have field paths for lists",
			expect: []string{"config.image", "config.args"},
			message: &tasksv1.Task{
				Config: &tasksv1.Config{
					Image: "docker.io/prometheus/prom:latest",
					Args:  []string{"--debug", "--insecure"},
				},
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			empty := proto.Clone(tt.message)
			ClearProto(empty.ProtoReflect())
			paths := []string{}

			err := compareMessages(empty.ProtoReflect(), tt.message.ProtoReflect(), "", &paths)
			if err != nil {
				t.Fatal(err)
			}

			gotClone := slices.Clone(paths)
			expectClone := slices.Clone(tt.expect)

			slices.Sort(gotClone)
			slices.Sort(expectClone)

			assert.Equal(t, expectClone, gotClone)
		})
	}
}
