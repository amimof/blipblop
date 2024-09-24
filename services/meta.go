package services

import (
	"errors"
	"time"

	"github.com/amimof/blipblop/api/services/containers/v1"
	"github.com/amimof/blipblop/api/types/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type MetaObject interface {
	GetMeta() *types.Meta
}

var ErrNoName = errors.New("object has no name")

// EnsureMetaForContainer ensures that appropriate meta is present for the provided container.
// Such as created/updated timestamp, revision etc.
func EnsureMetaForContainer(c *containers.Container) error {
	if c.GetMeta() == nil {
		return ErrNoName
	}

	if c.GetMeta().Created == nil {
		c.Meta.Created = timestamppb.New(time.Now())
	}

	if c.Meta.Updated == nil {
		c.Meta.Updated = timestamppb.New(time.Now())
	}

	if c.Meta.Revision < 1 {
		c.Meta.Revision = 1
	}

	return nil
}

func EnsureMeta(entity MetaObject) error {
	if entity.GetMeta() == nil {
		return ErrNoName
	}

	m := entity.GetMeta()

	if m.GetCreated() == nil {
		m.Created = timestamppb.New(time.Now())
	}

	if m.GetUpdated() == nil {
		m.Updated = timestamppb.New(time.Now())
	}

	if m.GetRevision() < 1 {
		m.Revision = 1
	}

	return nil
}
