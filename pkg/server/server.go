package server

import (
	"io"
	"net"

	"github.com/amimof/blipblop/pkg/logger"
	"github.com/amimof/blipblop/services"
	"github.com/amimof/blipblop/services/container"
	"github.com/amimof/blipblop/services/event"
	"github.com/amimof/blipblop/services/node"

	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/keepalive"
)

func init() {
	grpclog.SetLoggerV2(grpclog.NewLoggerV2(io.Discard, io.Discard, io.Discard))
}

type Server struct {
	grpcOpts   []grpc.ServerOption
	grpcServer *grpc.Server
	addr       *string
	logger     logger.Logger

	// services
	eventService     *event.EventService
	containerService *container.ContainerService
	nodeService      *node.NodeService
}

type NewServerOption func(*Server)

func WithLogger(lgr logger.Logger) NewServerOption {
	return func(s *Server) {
		s.logger = lgr
	}
}

// func WithEventServiceRepo(repo repository.Repository) NewServerOption {
// 	return func(s *Server) {
// 		s.eventService = event.NewService(repo)
// 	}
// }
//
// func WithNodeServiceRepo(repo repository.Repository) NewServerOption {
// 	return func(s *Server) {
// 		s.nodeService = node.NewService(repo, s.svc.eventService)
// 	}
// }
//
// func WithContainerServiceRepo(repo repository.Repository) NewServerOption {
// 	return func(s *Server) {
// 		s.containerService = container.NewService(repo, s.svc.eventService)
// 	}
// }

func WithGrpcOption(opts ...grpc.ServerOption) NewServerOption {
	return func(s *Server) {
		s.grpcOpts = opts
	}
}

func New(addr string, opts ...NewServerOption) *Server {
	grpcOpts := []grpc.ServerOption{
		grpc.KeepaliveParams(keepalive.ServerParameters{}),
	}

	// Setup events
	// eventService := event.NewService(repository.NewInMemRepo())

	// Setup server
	server := &Server{
		grpcOpts: grpcOpts,
		addr:     &addr,
		logger:   logger.ConsoleLogger{},
		// eventService:     eventService,
		// containerService: container.NewService(repository.NewInMemRepo(), eventService),
		// nodeService:      node.NewService(repository.NewInMemRepo(), eventService),
	}

	// Apply options
	for _, opt := range opts {
		opt(server)
	}

	// Setup gRPC server and register services
	server.grpcServer = grpc.NewServer(server.grpcOpts...)
	// server.grpcServer.RegisterService(&events.EventService_ServiceDesc, server.svc.eventService)
	// server.grpcServer.RegisterService(&nodes.NodeService_ServiceDesc, server.svc.nodeService)
	// server.grpcServer.RegisterService(&containers.ContainerService_ServiceDesc, server.svc.containerService)

	return server
}

func (s *Server) Serve(lis net.Listener) error {
	return s.grpcServer.Serve(lis)
}

func (s *Server) Addr() string {
	return *s.addr
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

// func (s *Server) registerServices(srv *grpc.Server) *services {
// 	// Events
// 	eventService := event.NewService(event.NewInMemRepo())
// 	srv.RegisterService(&events.EventService_ServiceDesc, eventService)
//
// 	// Containers
// 	containerService := container.NewService(container.NewRedisRepo(), eventService)
// 	srv.RegisterService(&containers.ContainerService_ServiceDesc, containerService)
//
// 	// Nodes
// 	nodeService := node.NewService(node.NewInMemRepo(), eventService)
// 	srv.RegisterService(&nodes.NodeService_ServiceDesc, nodeService)
//
// 	return &services{
// 		nodeService:      nodeService,
// 		eventService:     eventService,
// 		containerService: containerService,
// 	}
// }
