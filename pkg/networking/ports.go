package networking

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	gocni "github.com/containerd/go-cni"

	tasksv1 "github.com/amimof/voiyd/api/services/tasks/v1"
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

func (p *PortMapping) String() string {
	return fmt.Sprintf("%d:%d", p.Source, p.Destination)
}

func ParseCNIPortMapping(pm *tasksv1.PortMapping) gocni.PortMapping {
	return gocni.PortMapping{
		HostPort:      int32(pm.GetHostPort()),
		ContainerPort: int32(pm.GetTargetPort()),
		Protocol:      pm.GetProtocol(),
		HostIP:        pm.GetHostIp(),
	}
}

func ParseCNIPortMappings(pm ...*tasksv1.PortMapping) []gocni.PortMapping {
	mappings := make([]gocni.PortMapping, len(pm))
	for i, mapping := range pm {
		if mapping.Protocol == "" {
			mapping.Protocol = "TCP"
		}
		mappings[i] = ParseCNIPortMapping(mapping)
	}
	return mappings
}

func ParsePortMapping(pm gocni.PortMapping) *tasksv1.PortMapping {
	return &tasksv1.PortMapping{
		HostPort:   uint32(pm.HostPort),
		TargetPort: uint32(pm.ContainerPort),
		Protocol:   pm.Protocol,
		HostIp:     pm.HostIP,
		Name:       fmt.Sprintf("%d:%d", pm.HostPort, pm.ContainerPort),
	}
}

func ParsePortMappings(pm ...gocni.PortMapping) []*tasksv1.PortMapping {
	mappings := make([]*tasksv1.PortMapping, len(pm))
	for i, mapping := range pm {
		if mapping.Protocol == "" {
			mapping.Protocol = "TCP"
		}
		mappings[i] = ParsePortMapping(mapping)
	}
	return mappings
}
