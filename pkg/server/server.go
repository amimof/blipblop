// Package server provides types that creates and runs server instances
package server

import (
	"io"
	"net"

	"github.com/amimof/voiyd/pkg/logger"
	"github.com/amimof/voiyd/services"

	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
)

func init() {
	grpclog.SetLoggerV2(grpclog.NewLoggerV2(io.Discard, io.Discard, io.Discard))
}

type Server struct {
	grpcOpts   []grpc.ServerOption
	grpcServer *grpc.Server
	logger     logger.Logger
}

type NewServerOption func(*Server)

func WithLogger(lgr logger.Logger) NewServerOption {
	return func(s *Server) {
		s.logger = lgr
	}
}

func WithGrpcOption(opts ...grpc.ServerOption) NewServerOption {
	return func(s *Server) {
		s.grpcOpts = append(s.grpcOpts, opts...)
	}
}

func New(opts ...NewServerOption) (*Server, error) {
	grpcOpts := []grpc.ServerOption{
		grpc.KeepaliveParams(keepalive.ServerParameters{}),
	}

	// Setup server
	server := &Server{
		grpcOpts: grpcOpts,
		logger:   logger.ConsoleLogger{},
	}

	// Apply options
	for _, opt := range opts {
		opt(server)
	}

	server.grpcServer = grpc.NewServer(server.grpcOpts...)

	reflection.Register(server.grpcServer)

	return server, nil
}

func (s *Server) Serve(lis net.Listener) error {
	return s.grpcServer.Serve(lis)
}

func (s *Server) Shutdown() {
	s.grpcServer.GracefulStop()
}

func (s *Server) ForceShutdown() {
	s.grpcServer.Stop()
}

func (s *Server) RegisterService(svcs ...services.Service) error {
	for _, svc := range svcs {
		err := svc.Register(s.grpcServer)
		if err != nil {
			return err
		}
	}
	return nil
}
