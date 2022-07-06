package models

type Node struct {
	Metadata
	Status *NodeStatus `json:"status,omitempty"`
}

type NodeStatus struct {
	IPs      []string `json:"ips,omitempty"`
	HostName string   `json:"hostname,omitempty"`
	Arch     string   `json:"arch,omitempty"`
	Os       string   `json:"os,omitempty"`
}
