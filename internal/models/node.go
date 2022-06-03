package models 

import (
	"github.com/amimof/blipblop/pkg/labels"
)

type Node struct {
	Name *string `json:"name,omitempty"`
	Labels  labels.Label
}