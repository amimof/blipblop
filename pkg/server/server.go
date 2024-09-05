package server

import (
	"io"
	"net"

	"github.com/amimof/blipblop/api/services/containers/v1"
	"github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/api/services/nodes/v1"

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
	srv  *grpc.Server
	addr *string
	svc  *services
}

type services struct {
	eventService     *event.EventService
	containerService *container.ContainerService
	nodeService      *node.NodeService
}

func New(addr string) *Server {
	opts := []grpc.ServerOption{
		grpc.KeepaliveParams(keepalive.ServerParameters{}),
	}
	srv := grpc.NewServer(opts...)
	svc := registerServices(srv)
	return &Server{
		srv:  srv,
		addr: &addr,
		svc:  svc,
	}
}

func (s *Server) Serve(lis net.Listener) error {
	return s.srv.Serve(lis)
}

func (s *Server) Addr() string {
	return *s.addr
}

func (s *Server) Shutdown() {
	s.srv.GracefulStop()
}

func (s *Server) ForceShutdown() {
	s.srv.Stop()
}

func registerServices(srv *grpc.Server) *services {
	// Events
	eventService := event.NewService(event.NewInMemRepo())
	srv.RegisterService(&events.EventService_ServiceDesc, eventService)

	// Containers
	containerService := container.NewService(container.NewInMemRepo(), eventService)
	srv.RegisterService(&containers.ContainerService_ServiceDesc, containerService)

	// Nodes
	nodeService := node.NewService(node.NewInMemRepo(), eventService)
	srv.RegisterService(&nodes.NodeService_ServiceDesc, nodeService)

	return &services{
		nodeService:      nodeService,
		eventService:     eventService,
		containerService: containerService,
	}
}
