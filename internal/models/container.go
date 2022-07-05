package models

import (
	"github.com/amimof/blipblop/pkg/labels"
	"github.com/containerd/containerd"
	"github.com/containerd/go-cni"
	"github.com/opencontainers/runtime-spec/specs-go"
	"net"
	"time"
)

type Container struct {
	Name    *string           `json:"name,omitempty"`
	Image   *string           `json:"image,omitempty"`
	Digest  string            `json:"digest,omitempty"`
	Args    []string          `json:"args,omitempty"`
	EnvVars []string          `json:"env_vars,omitempty"`
	Mounts  []specs.Mount     `json:"mounts,omitempty"`
	Ports   []cni.PortMapping `json:"ports,omitempty"`
	Labels  labels.Label      `json:"labels,omitempty"`
	Status  *Status           `json:"status,omitempty"`
	Created time.Time         `json:"created,omitempty`
	Updated time.Time         `json:"updated,omitempty`
}

type Status struct {
	Network *NetworkStatus `json:"network,omitempty"`
	Runtime *RuntimeStatus `json:"runtime,omitempty"`
}

type RuntimeStatus struct {
	Status containerd.ProcessStatus `json:"status,omitempty"`
}

type NetworkStatus struct {
	IP  net.IP `json:"ip,omitempty"`
	GW  net.IP `json:"gw,omitempty"`
	Mac string `json:"mac,omitempty"`
}
