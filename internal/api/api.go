package api

import (
	"context"
	containersv1 "github.com/amimof/blipblop/api/services/containers/v1"
	eventsv1 "github.com/amimof/blipblop/api/services/events/v1"
	nodesv1 "github.com/amimof/blipblop/api/services/nodes/v1"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"io/ioutil"
	"net/http"
	"os"
	//"github.com/amimof/blipblop/internal/controller"
	//"github.com/amimof/blipblop/internal/handlers"
	//"github.com/amimof/blipblop/internal/repo"
	//"github.com/amimof/blipblop/internal/routes"
	"github.com/amimof/blipblop/internal/services"
	//"github.com/amimof/blipblop/pkg/client"
	//"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"google.golang.org/grpc/credentials/insecure"
	//"google.golang.org/grpc/credentials"
	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"
)

type APIv1 struct {
	app        *fiber.App
	router     fiber.Router
	root       string
	grpcServer *grpc.Server
}

// func (a *APIv1) setupHandlers(client *client.LocalClient) *APIv1 {
// 	// Containers
// 	routes.MapContainerRoutes(a.router.Group("/containers"), handlers.NewContainerHandler(controller.NewContainerController(client, repo.NewInMemContainerRepo())))
// 	// Nodes
// 	routes.MapNodeRoutes(a.router.Group("/nodes"), handlers.NewNodeHandler(controller.NewNodeController(client)))
// 	return a
// }

// func (a *APIv1) Handler() http.Handler {
// 	return adaptor.FiberApp(a.app)
// }

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

	//opts := []grpc.DialOption{grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(insecure.CertPool, ""))}
	// This is where the gRPC-Gateway proxies the requests.
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
	err = containersv1.RegisterContainerServiceHandler(ctx, mux, conn)
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

	// Setup our internal client
	nodeService := services.NewNodeService()
	eventService := services.NewEventService()
	containerService := services.NewContainerService()
	//client := client.NewLocalClient(nodeService, eventService, containerService)

	// Register services
	nodesv1.RegisterNodeServiceServer(grpcServer, nodeService)
	eventsv1.RegisterEventServiceServer(grpcServer, eventService)
	containersv1.RegisterContainerServiceServer(grpcServer, containerService)

	// Setup http api
	// app := fiber.New()
	// router := app.Group("/api/v1/")
	api := &APIv1{
		//app:        app,
		//router:     router,
		//root:       "/api/v1/",
		grpcServer: grpcServer,
	}
	// return api.setupHandlers(client)
	return api
}
