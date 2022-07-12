package api

import (
	"context"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/amimof/blipblop/services/container"
	"github.com/amimof/blipblop/services/event"
	"github.com/amimof/blipblop/services/node"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/grpclog"
)

type APIv1 struct {
	grpcServer       *grpc.Server
	containerService *container.ContainerService
	nodeService      *node.NodeService
	eventService     *event.EventService
}

func (a *APIv1) GrpcServer() *grpc.Server {
	return a.grpcServer
}

func (a *APIv1) Run(addr string) error {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	mux := runtime.NewServeMux()

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
	err = a.eventService.RegisterHandler(ctx, mux, conn)
	if err != nil {
		return err
	}
	err = a.containerService.RegisterHandler(ctx, mux, conn)
	if err != nil {
		return err
	}
	err = a.nodeService.RegisterHandler(ctx, mux, conn)
	if err != nil {
		return err
	}
	return http.ListenAndServe(":8443", mux)
}

func NewAPIv1() *APIv1 {
	// Setup grpc services
	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)

	// Events
	eventService := event.NewService(event.NewInMemRepo())
	eventService.Register(grpcServer)

	// Containers
	containerService := container.NewService(container.NewInMemRepo(), eventService)
	containerService.Register(grpcServer)

	// Nodes
	nodeService := node.NewService(node.NewInMemRepo(), eventService)
	nodeService.Register(grpcServer)

	// Register services
	//eventService
	//eventsv1.RegisterEventServiceServer(grpcServer, event.NewEventService())

	// Setup http api
	return &APIv1{
		grpcServer:       grpcServer,
		containerService: containerService,
		nodeService:      nodeService,
		eventService:     eventService,
	}
}
