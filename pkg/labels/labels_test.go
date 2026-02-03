package labels

import (
	"testing"

	nodesv1 "github.com/amimof/voiyd/api/services/nodes/v1"
	"github.com/amimof/voiyd/api/types/v1"
	"github.com/stretchr/testify/assert"
)

func Test_CompositeSelector(t *testing.T) {
	node := nodesv1.Node{
		Meta: &types.Meta{
			Name: "node-1",
			Labels: map[string]string{
				"hostname": "node-1.foo.com",
				"os":       "linux",
				"arch":     "arm64",
			},
			Generation: 1,
		},
		Config: &nodesv1.Config{},
		Status: &nodesv1.Status{},
	}

	tests := []struct {
		name   string
		input  Label
		expect bool
	}{
		{
			name: "hostname selector should match",
			input: map[string]string{
				"hostname": "node-1.foo.com",
			},
			expect: true,
		},
		{
			name:   "empty selector should match",
			input:  map[string]string{},
			expect: true,
		},
		{
			name:   "nil selector should match",
			input:  nil,
			expect: true,
		},
		{
			name: "missing field in selector should'nt match",
			input: map[string]string{
				"role": "production",
			},
			expect: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			selector := NewCompositeSelectorFromMap(test.input)
			matches := selector.Matches(node.GetMeta().GetLabels())
			assert.Equal(t, test.expect, matches, "selector didn't match expectation")
		})
	}
}
