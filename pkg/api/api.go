package api

import (
	//"log"
	"github.com/amimof/blipblop/internal/handlers"
	"github.com/amimof/blipblop/internal/repo"
	"github.com/amimof/blipblop/internal/routes"
	"github.com/amimof/blipblop/internal/services"
	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"net/http"
)

type APIv1 struct {
	app    *fiber.App
	router fiber.Router
	root   string
}

func (a *APIv1) setupHandlers() *APIv1 {
	// Units
	routes.MapUnitRoutes(a.router.Group("/units"), handlers.NewUnitHandler(services.NewUnitService(repo.NewInMemUnitRepo())))
	// Nodes
	routes.MapNodeRoutes(a.router.Group("/nodes"), handlers.NewNodeHandler(services.NewNodeService(repo.NewNodeRepo())))
	return a
}

func (a *APIv1) Handler() http.Handler {
	return adaptor.FiberApp(a.app)
}

func NewAPIv1() *APIv1 {
	app := fiber.New()
	router := app.Group("/api/v1/")
	api := &APIv1{
		app:    app,
		router: router,
		root:   "/api/v1/",
	}
	return api.setupHandlers()
}

func (a *APIv1) Handler() http.Handler {
	return adaptor.FiberApp(a.app)
}
