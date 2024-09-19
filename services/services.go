package services

import "google.golang.org/grpc"

type Service interface {
	Register(*grpc.Server) error
}
