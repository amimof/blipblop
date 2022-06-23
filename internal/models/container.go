package models

import (
	//"github.com/google/uuid"
	"bytes"
	"encoding/gob"
	"github.com/amimof/blipblop/pkg/labels"
	"github.com/containerd/containerd"
	"github.com/containerd/go-cni"
	"github.com/opencontainers/runtime-spec/specs-go"
	"net"
)

type Container struct {
	//UUID			uuid.UUID `json:"uuid,omitempty"`
	Name    *string           `json:"name,omitempty"`
	Image   *string           `json:"image,omitempty"`
	Args    []string          `json:"args,omitempty"`
	EnvVars []string          `json:"env_vars,omitempty"`
	Mounts  []specs.Mount     `json:"mounts,omitempty"`
	Ports   []cni.PortMapping `json"ports,omitempty"`
	Labels  labels.Label
	Status  *Status `json:"status,omitempty"`
	//Container *containers.Container `json:"container,omitempty"`
	//Network		*cni.Result `json:"container,omitempty"`
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

func (u *Container) Encode() ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	return buf.Bytes(), enc.Encode(u)
}
