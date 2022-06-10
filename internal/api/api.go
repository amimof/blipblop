package api

import (
	//"log"
	"github.com/amimof/blipblop/internal/handlers"
	"github.com/amimof/blipblop/internal/repo"
	"github.com/amimof/blipblop/internal/routes"
	"github.com/amimof/blipblop/internal/services"
	proto "github.com/amimof/blipblop/proto"
	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"google.golang.org/grpc"
	"net/http"
)

type APIv1 struct {
	app        *fiber.App
	router     fiber.Router
	root       string
	grpcServer *grpc.Server
}

func (a *APIv1) setupHandlers() *APIv1 {
	// Units
	routes.MapUnitRoutes(a.router.Group("/units"), handlers.NewUnitHandler(services.NewUnitService(repo.NewInMemUnitRepo())))
	// Nodes
	routes.MapNodeRoutes(a.router.Group("/nodes"), handlers.NewNodeHandler(services.NewNodeService()))
	return a
}

func (a *APIv1) Handler() http.Handler {
	return adaptor.FiberApp(a.app)
}

func (a *APIv1) GrpcServer() *grpc.Server {
	return a.grpcServer
}

func NewAPIv1() *APIv1 {
	// Setup grpc services
	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)
	nodeService := services.NewNodeService()
	proto.RegisterNodeServiceServer(grpcServer, nodeService)
	// Setup http api
	app := fiber.New()
	router := app.Group("/api/v1/")
	api := &APIv1{
		app:        app,
		router:     router,
		root:       "/api/v1/",
		grpcServer: grpcServer,
	}
	return api.setupHandlers()
}
