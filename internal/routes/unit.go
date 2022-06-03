package routes

import (
<<<<<<< HEAD
	"github.com/gofiber/fiber/v2"
	"github.com/amimof/blipblop/internal/handlers"
=======
	"github.com/amimof/blipblop/internal/handlers"
	"github.com/gofiber/fiber/v2"
>>>>>>> 4483218 (Split server and node:)
)

func MapUnitRoutes(group fiber.Router, h handlers.UnitHandler) {
	group.Get("/:id", h.Get())
	group.Get("/", h.GetAll())
	group.Post("/", h.Create())
	group.Put("/:id", h.Update())
	group.Delete("/:id", h.Delete())
	group.Put("/:id/start", h.Start())
	group.Put("/:id/stop", h.Stop())
}