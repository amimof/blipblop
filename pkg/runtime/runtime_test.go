package runtime

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Runtime_ContainerID(t *testing.T) {
	for range 100 {
		id := GenerateID()
		assert.Len(t, id, 64, "length should be 64")
	}
}
