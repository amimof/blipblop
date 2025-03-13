package protoutils

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	containersv1 "github.com/amimof/blipblop/api/services/containers/v1"
	metav1 "github.com/amimof/blipblop/api/types/v1"
	jsonpatch "github.com/evanphx/json-patch"
)

func TestImmutableFields(t *testing.T) {
	existingContainer := &containersv1.Container{
		Meta: &metav1.Meta{
			Name: "test-container",
		},
		Config: &containersv1.Config{
			Image: "test-image-new",
			PortMappings: []*containersv1.PortMapping{
				{
					HostPort:      8080,
					ContainerPort: 8080,
					Protocol:      "TCP",
				},
			},
		},
		Status: &containersv1.Status{
			Phase: "idle",
			Node:  "bbnode",
		},
	}

	patchContainer := &containersv1.Container{
		// Meta: &metav1.Meta{
		// 	Name: "test-container-new",
		// },
		Config: &containersv1.Config{
			Image: "test-image-new",
			PortMappings: []*containersv1.PortMapping{
				{
					HostPort:      443,
					ContainerPort: 8080,
					Protocol:      "TCP",
				},
				{
					HostPort:      8080,
					ContainerPort: 8080,
					Protocol:      "TCP",
				},
			},
		},
		// Status: &containersv1.Status{
		// 	Phase: "running",
		// 	Node:  "bbmaster",
		// },
	}

	// maskedUpdate, err := ApplyFieldMaskToNewMessage(updateContainer, &fieldmaskpb.FieldMask{})
	// if err != nil {
	// 	t.Errorf("error %v", err)
	// }
	//
	// fmt.Printf("update: %+v\n", maskedUpdate)
	//
	// proto.Merge(existingContainer, updateContainer)
	// //
	// fmt.Printf("%+v\n", existingContainer)
	//
	// proto.Merge(updateContainer, immutableUpdate)
	// fmt.Printf("%+v\n", updateContainer)

	// fmt.Printf("They are same? %t", reflect.DeepEqual(existingContainer.GetConfig(), updateContainer.GetConfig()))

	updated, err := StrategicMergePatch(existingContainer, patchContainer)
	if err != nil {
		t.Error(err)
	}

	t.Logf("Result: %+v", updated)
}

func StrategicMergePatch(target, patch *containersv1.Container) (*containersv1.Container, error) {
	targetb, err := json.Marshal(target)
	if err != nil {
		return nil, err
	}

	patchb, err := json.Marshal(patch)
	if err != nil {
		return nil, err
	}

	// p, err := jsonpatch.CreateMergePatch(targetb, patchb)
	// if err != nil {
	// 	return nil, err
	// }

	b, err := jsonpatch.MergePatch(targetb, patchb)
	if err != nil {
		return nil, err
	}

	var c containersv1.Container
	err = json.Unmarshal(b, &c)
	if err != nil {
		return nil, err
	}

	return &c, nil
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
