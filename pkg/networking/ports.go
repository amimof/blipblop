package networking

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/amimof/blipblop/api/services/containers/v1"
	gocni "github.com/containerd/go-cni"
)

type PortMapping struct {
	Source      uint32
	Destination uint32
}

var errInvalidPortMapping = errors.New("invalid port mapping")

func ParsePorts(p string) (*PortMapping, error) {
	split := strings.Split(p, ":")
	if len(split) != 2 {
		return nil, errInvalidPortMapping
	}

	src, err := strconv.Atoi(split[0])
	if err != nil {
		return nil, errInvalidPortMapping
	}
	dst, err := strconv.Atoi(split[1])
	if err != nil {
		return nil, errInvalidPortMapping
	}

	return &PortMapping{uint32(src), uint32(dst)}, nil
}

func (p *PortMapping) String() {
	fmt.Printf("%d:%d", p.Source, p.Destination)
}

func ParseCNIPortMapping(pm *containers.PortMapping) gocni.PortMapping {
	return gocni.PortMapping{
		HostPort:      int32(pm.GetHostPort()),
		ContainerPort: int32(pm.GetContainerPort()),
		Protocol:      pm.GetProtocol(),
		HostIP:        pm.GetHostIP(),
	}
}

func ParseCNIPortMappings(pm ...*containers.PortMapping) []gocni.PortMapping {
	mappings := make([]gocni.PortMapping, len(pm))
	for i, mapping := range pm {
		if mapping.Protocol == "" {
			mapping.Protocol = "TCP"
		}
		mappings[i] = ParseCNIPortMapping(mapping)
	}
	return mappings
}
