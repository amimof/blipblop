package services

import (
	"github.com/amimof/blipblop/api/types/v1"
	"google.golang.org/grpc"
)

type MetaObject interface {
	GetMeta() *types.Meta
}

type Service interface {
	Register(*grpc.Server) error
}
