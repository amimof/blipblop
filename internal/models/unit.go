package models

import (
	//"github.com/google/uuid"
<<<<<<< HEAD
	"github.com/containerd/containerd"
	"github.com/containerd/go-cni"
	"github.com/amimof/blipblop/pkg/labels"
=======
	"github.com/amimof/blipblop/pkg/labels"
	"github.com/containerd/containerd"
	"github.com/containerd/go-cni"
>>>>>>> 4483218 (Split server and node:)
	"github.com/opencontainers/runtime-spec/specs-go"
	"net"
)

type Unit struct {
	//UUID			uuid.UUID `json:"uuid,omitempty"`
<<<<<<< HEAD
	Name			*string `json:"name,omitempty"`
	Image 		*string `json:"image,omitempty"`
	Args			[]string `json:"args,omitempty"`
	EnvVars   []string `json:"env_vars,omitempty"`
	Mounts		[]specs.Mount `json:"mounts,omitempty"`
	Ports			[]cni.PortMapping `json"ports,omitempty"`
	Labels		labels.Label
	Status		*Status `json:"status,omitempty"`
=======
	Name    *string           `json:"name,omitempty"`
	Image   *string           `json:"image,omitempty"`
	Args    []string          `json:"args,omitempty"`
	EnvVars []string          `json:"env_vars,omitempty"`
	Mounts  []specs.Mount     `json:"mounts,omitempty"`
	Ports   []cni.PortMapping `json"ports,omitempty"`
	Labels  labels.Label
	Status  *Status `json:"status,omitempty"`
>>>>>>> 4483218 (Split server and node:)
	//Container *containers.Container `json:"container,omitempty"`
	//Network		*cni.Result `json:"container,omitempty"`
}

type Status struct {
	Network *NetworkStatus `json:"network,omitempty"`
<<<<<<< HEAD
	Runtime	*RuntimeStatus `json:"runtime,omitempty"`
=======
	Runtime *RuntimeStatus `json:"runtime,omitempty"`
>>>>>>> 4483218 (Split server and node:)
}

type RuntimeStatus struct {
	Status containerd.ProcessStatus `json:"status,omitempty"`
}

type NetworkStatus struct {
<<<<<<< HEAD
	IP net.IP `json:"ip,omitempty"`
	GW net.IP `json:"gw,omitempty"`
	Mac string `json:"mac,omitempty"`
}
=======
	IP  net.IP `json:"ip,omitempty"`
	GW  net.IP `json:"gw,omitempty"`
	Mac string `json:"mac,omitempty"`
}
>>>>>>> 4483218 (Split server and node:)
