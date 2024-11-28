package repository

import (
	"context"
	"testing"

	"github.com/amimof/blipblop/api/services/containers/v1"
	"github.com/amimof/blipblop/api/types/v1"
	"github.com/amimof/blipblop/pkg/labels"
	"github.com/stretchr/testify/assert"
)

func initInMemContainerRepo(ctx context.Context, repo ContainerRepository) (ContainerRepository, error) {
	ctrs := []*containers.Container{
		{
			Meta: &types.Meta{
				Name: "container-without-labels",
			},
		},
		{
			Meta: &types.Meta{
				Name: "container-with-labels",
				Labels: map[string]string{
					"app": "default",
				},
			},
		},
		{
			Meta: &types.Meta{
				Name: "container-with-multiple-labels",
				Labels: map[string]string{
					"app":                       "backend",
					"region":                    "west",
					"blipblop.io/container-set": "test-set",
				},
			},
		},
	}

	for _, ctr := range ctrs {
		err := repo.Create(ctx, ctr)
		if err != nil {
			return nil, err
		}
	}

	return repo, nil
}

func TestListContainersWithFilter(t *testing.T) {
	ctx := context.Background()
	filter := labels.New()
	filter.Set("app", "default")

	repo, err := initInMemContainerRepo(ctx, NewContainerInMemRepo())
	assert.NoError(t, err)

	ctrs, err := repo.List(ctx, filter)
	if err != nil {
		t.Fatal(err)
	}

	expected := "container-with-labels"

	assert.Len(t, ctrs, 1, "length of containers should be 1")

	for _, ctr := range ctrs {
		assert.Equal(t, ctr.GetMeta().GetName(), expected, "containers should match")
	}
}

func TestListContainersWithNoFilter(t *testing.T) {
	ctx := context.Background()

	repo, err := initInMemContainerRepo(ctx, NewContainerInMemRepo())
	assert.NoError(t, err)

	ctrs, err := repo.List(ctx)
	if err != nil {
		t.Fatal(err)
	}

	assert.Len(t, ctrs, 3, "length of containers should be 3")
}
