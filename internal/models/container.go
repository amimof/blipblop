package models

import (
	"github.com/amimof/blipblop/pkg/labels"
	"github.com/containerd/go-cni"
	"github.com/opencontainers/runtime-spec/specs-go"
	"net"
	"time"
)

type Container struct {
	Metadata
	Status *ContainerStatus `json:"status,omitempty"`
	Config *ContainerConfig `json:"config,omitempty"`
}

type ContainerConfig struct {
	Image   *string           `json:"image,omitempty"`
	Args    []string          `json:"args,omitempty"`
	EnvVars []string          `json:"env_vars,omitempty"`
	Mounts  []specs.Mount     `json:"mounts,omitempty"`
	Ports   []cni.PortMapping `json:"ports,omitempty"`
}

type Metadata struct {
	Name     *string      `json:"name,omitempty"`
	Created  time.Time    `json:"created,omitempty`
	Updated  time.Time    `json:"updated,omitempty`
	Labels   labels.Label `json:"labels,omitempty"`
	Revision uint64       `json:"revision,omitempty"`
}

type ContainerStatus struct {
	State string `json:"state,omitempty"`
	Node  string `json:"node,omitempty"`
	IP    net.IP `json:"ip,omitempty"`
}
