package routes

import (
	"github.com/amimof/blipblop/internal/handlers"
	"github.com/gofiber/fiber/v2"
)

func MapContainerRoutes(group fiber.Router, h handlers.ContainerHandler) {
	group.Get("/:id", h.Get())
	group.Get("/", h.GetAll())
	group.Post("/", h.Create())
	group.Put("/:id", h.Update())
	group.Delete("/:id", h.Delete())
	group.Put("/:id/start", h.Start())
	group.Put("/:id/stop", h.Stop())
}
