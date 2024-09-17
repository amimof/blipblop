package server

import (
	"fmt"
	"io"
	"net"

	"github.com/amimof/blipblop/api/services/containers/v1"
	"github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/api/services/nodes/v1"
	"github.com/amimof/blipblop/pkg/repository"
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
	svc        *services
}

type NewServerOption func(*Server)

type services struct {
	eventService     *event.EventService
	containerService *container.ContainerService
	nodeService      *node.NodeService
}

func WithEventServiceRepo(repo repository.Repository) NewServerOption {
	return func(s *Server) {
		s.svc.eventService = event.NewService(repo)
	}
}

func WithNodeServiceRepo(repo repository.Repository) NewServerOption {
	return func(s *Server) {
		s.svc.nodeService = node.NewService(repo, s.svc.eventService)
	}
}

func WithContainerServiceRepo(repo repository.Repository) NewServerOption {
	return func(s *Server) {
		fmt.Println("Setting repo to ", repo)
		s.svc.containerService = container.NewService(repo, s.svc.eventService)
	}
}

func WithGrpcOption(opts ...grpc.ServerOption) NewServerOption {
	return func(s *Server) {
		s.grpcOpts = opts
	}
}

func New(addr string, opts ...NewServerOption) *Server {
	grpcOpts := []grpc.ServerOption{
		grpc.KeepaliveParams(keepalive.ServerParameters{}),
	}

	//      srv.RegisterService(&events.EventService_ServiceDesc, eventService)////      // Containers//      containerService := container.NewService(container.NewRedisRepo(), eventService)//      srv.RegisterService(&containers.ContainerService_ServiceDesc, containerService)////      // Nodes//      nodeService := node.NewService(node.NewInMemRepo(), eventService)//      srv.RegisterService(&nodes.NodeService_ServiceDesc, nodeService)

	// Setup services
	eventService := event.NewService(repository.NewInMemRepo())
	containerService := container.NewService(repository.NewInMemRepo(), eventService)
	nodeService := node.NewService(repository.NewInMemRepo(), eventService)

	svc := &services{
		eventService:     eventService,
		containerService: containerService,
		nodeService:      nodeService,
	}

	server := &Server{
		grpcOpts: grpcOpts,
		addr:     &addr,
		svc:      svc,
	}

	// Apply options
	for _, opt := range opts {
		opt(server)
	}

	// Setup gRPC server and register services
	server.grpcServer = grpc.NewServer(server.grpcOpts...)
	server.grpcServer.RegisterService(&events.EventService_ServiceDesc, server.svc.eventService)
	server.grpcServer.RegisterService(&nodes.NodeService_ServiceDesc, server.svc.nodeService)
	server.grpcServer.RegisterService(&containers.ContainerService_ServiceDesc, server.svc.containerService)

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
