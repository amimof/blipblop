package networking

import (
	"fmt"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPortMapping(t *testing.T) {
	invalidMappings := []string{"8080", ":8080", "8080:", " ", ""}

	for _, invalidMapping := range invalidMappings {
		_, err := ParsePorts(invalidMapping)
		if !assert.EqualError(t, err, "invalid port mapping") {
			log.Fatal(err)
		}
	}

	validPortMapping := "9090:443"
	pm, err := ParsePorts(validPortMapping)
	if err != nil {
		log.Fatal(err)
	}
	if !assert.Equal(t, uint32(9090), pm.Source) {
		log.Println(fmt.Errorf("expected source port to be %d, got %d", 9090, pm.Source))
	}
	if !assert.Equal(t, uint32(443), pm.Destination) {
		log.Println(fmt.Errorf("expected destination port to be %d, got %d", 443, pm.Destination))
	}
}
