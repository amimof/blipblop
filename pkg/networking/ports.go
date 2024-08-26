package networking

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
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
