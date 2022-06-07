package models

import (
	"github.com/amimof/blipblop/pkg/labels"
	//"github.com/gogo/protobuf/types"
)

type Node struct {
	Name   *string `json:"name,omitempty"`
	Labels labels.Label
	Created string
}