package api

import (
	"context"
	"io/ioutil"
	"net/http"
	"os"

	//containersv1 "github.com/amimof/blipblop/api/services/containers/v1"
	eventsv1 "github.com/amimof/blipblop/api/services/events/v1"
	nodesv1 "github.com/amimof/blipblop/api/services/nodes/v1"
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
	err = nodesv1.RegisterNodeServiceHandler(ctx, mux, conn)
	if err != nil {
		return err
	}
	err = a.containerService.RegisterHandler(ctx, mux, conn)
	if err != nil {
		return err
	}
	err = eventsv1.RegisterEventServiceHandler(ctx, mux, conn)
	if err != nil {
		return err
	}
	return http.ListenAndServe(":8443", mux)
}

func NewAPIv1() *APIv1 {
	// Setup grpc services
	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)

	// Setup services
	eventService := event.NewEventService()
	containerService := container.NewService(container.NewInMemRepo(), eventService)
	containerService.Register(grpcServer)

	// Register services
	nodesv1.RegisterNodeServiceServer(grpcServer, node.NewNodeService())
	eventsv1.RegisterEventServiceServer(grpcServer, event.NewEventService())
	//containersv1.RegisterContainerServiceServer(grpcServer, container.NewContainerService())

	// Setup http api
	return &APIv1{grpcServer: grpcServer, containerService: containerService}
}
