package models

import (
	"github.com/amimof/blipblop/pkg/labels"
	"time"
	//"github.com/gogo/protobuf/types"
)

type Node struct {
	Name    *string `json:"name,omitempty"`
	Labels  labels.Label
	Created string
}

type NodeStatus struct {
	LastSeenSeconds time.Duration
}
