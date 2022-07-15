package server

import (
	"context"
	"io/ioutil"
	"os"

	"github.com/amimof/blipblop/api/services/containers/v1"
	"github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/api/services/nodes/v1"

	"github.com/amimof/blipblop/services/container"
	"github.com/amimof/blipblop/services/event"
	"github.com/amimof/blipblop/services/node"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/grpclog"
)

type services struct {
	eventService     *event.EventService
	containerService *container.ContainerService
	nodeService      *node.NodeService
}

func RegisterServices(ctx context.Context, srv *grpc.Server, mux *runtime.ServeMux) error {
	svc := registerServices(srv)
	err := registerGateway(ctx, "localhost:5700", mux, svc)
	if err != nil {
		return nil
	}
	return nil
	//return http.ListenAndServe(fmt.Sprintf("%s:%d", s.TLSHost, s.TLSPort), mux)
}

func registerGateway(ctx context.Context, addr string, mux *runtime.ServeMux, svc *services) error {
	log := grpclog.NewLoggerV2(os.Stdout, ioutil.Discard, ioutil.Discard)
	grpclog.SetLoggerV2(log)

	conn, err := grpc.DialContext(
		ctx,
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return err
	}
	err = svc.eventService.RegisterHandler(ctx, mux, conn)
	if err != nil {
		return err
	}
	err = svc.containerService.RegisterHandler(ctx, mux, conn)
	if err != nil {
		return err
	}
	err = svc.nodeService.RegisterHandler(ctx, mux, conn)
	if err != nil {
		return err
	}
	return nil
}

func registerServices(srv *grpc.Server) *services {
	// Events
	eventService := event.NewService(event.NewInMemRepo())
	srv.RegisterService(&events.EventService_ServiceDesc, eventService)
	//eventService.Register(srv)

	// Containers
	containerService := container.NewService(container.NewInMemRepo(), eventService)
	srv.RegisterService(&containers.ContainerService_ServiceDesc, containerService)
	//containerService.Register(grpcServer)

	// Nodes
	nodeService := node.NewService(node.NewInMemRepo(), eventService)
	srv.RegisterService(&nodes.NodeService_ServiceDesc, nodeService)
	//nodeService.Register(grpcServer)

	return &services{
		nodeService:      nodeService,
		eventService:     eventService,
		containerService: containerService,
	}

}

// // TODO: Pass in server configuration to New()
// func New() *Server {
// 	// Setup grpc services
// 	var opts []grpc.ServerOption
// 	grpcServer := grpc.NewServer(opts...)

// 	// Setup http api
// 	return &Server{
// 		grpcServer:       grpcServer,
// 		containerService: containerService,
// 		nodeService:      nodeService,
// 		eventService:     eventService,
// 	}
// }
