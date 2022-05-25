package api

import (
	//"log"
	"net/http"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/adaptor/v2"
	"github.com/containerd/containerd"
	"github.com/amimof/blipblop/internal/routes"
	"github.com/amimof/blipblop/internal/handlers"
	"github.com/amimof/blipblop/internal/services"
	"github.com/amimof/blipblop/internal/repo"
	"github.com/containerd/go-cni"
)

type APIv1 struct {
	app *fiber.App
	router fiber.Router
	root string
	client *containerd.Client
	cni cni.CNI
}

func (a *APIv1) setupHandlers() *APIv1 {
	unitRepo := repo.NewUnitRepo(a.client, a.cni)
	unitService := services.NewUnitService(unitRepo)
	unitHandlers := handlers.NewUnitHandler(unitService)
	unitGroup := a.router.Group("/units")
	routes.MapUnitRoutes(unitGroup, unitHandlers)
	return a
	// api.router.Get("/", func(c *fiber.Ctx) error {
	// 	e := c.AcceptsEncodings("application/json")
	// 	log.Printf("%s", e)
	// 	return c.SendString("Home")
	// })
}

func NewAPIv1(client *containerd.Client, cni cni.CNI) *APIv1 {
	app := fiber.New()
	router := app.Group("/api/v1/")
	api := &APIv1{
		app: app,
		router: router,
		root: "/api/v1/",
		client: client,
		cni: cni,
	}
	return api.setupHandlers()
}

func (a *APIv1) Handler() http.Handler {
	return adaptor.FiberApp(a.app)
}