package handlers

import (
	"fmt"
	"github.com/amimof/blipblop/internal/controller"
	"github.com/amimof/blipblop/internal/models"
	"github.com/gofiber/fiber/v2"
)

type ContainerHandler interface {
	Get() fiber.Handler
	GetAll() fiber.Handler
	Create() fiber.Handler
	Update() fiber.Handler
	Delete() fiber.Handler
	Start() fiber.Handler
	Stop() fiber.Handler
}

type containerHandler struct {
	controller *controller.ContainerController
}

func (c containerHandler) Get() fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		unit, err := c.controller.Get(ctx.Params("id"))
		if err != nil {
			return ctx.Status(fiber.StatusInternalServerError).SendString(err.Error())
		}
		if unit != nil {
			return ctx.JSON(unit)
		}
		ctx.Status(fiber.StatusNotFound)
		return nil
	}
}
func (c containerHandler) GetAll() fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		units, err := c.controller.All()
		if err != nil {
			return ctx.Status(fiber.StatusInternalServerError).SendString(err.Error())
		}
		ctx.JSON(units)
		return nil
	}
}
func (c containerHandler) Create() fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		unit := new(models.Container)
		err := ctx.BodyParser(unit)
		if err != nil {
			return ctx.Status(fiber.StatusInternalServerError).SendString(err.Error())
		}
		// Check if already exists
		existing, err := c.controller.Get(*unit.Name)
		if err != nil {
			return ctx.Status(fiber.StatusInternalServerError).SendString(err.Error())
		} 
		if existing != nil {
			return ctx.Status(fiber.StatusConflict).SendString(fmt.Sprintf("failed to create container: %s already exists", *unit.Name))
		}
		err = c.controller.Create(unit)
		if err != nil {
			return ctx.Status(fiber.StatusInternalServerError).SendString(err.Error())
		}
		ctx.JSON(unit)
		return nil
	}
}
func (c containerHandler) Update() fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		id := ctx.Params("id")
		existing, err := c.controller.Get(id)
		if err != nil {
			return ctx.Status(fiber.StatusInternalServerError).SendString(err.Error())
		} 
		if existing == nil {
			return ctx.Status(fiber.StatusNotFound).SendString(fmt.Sprintf("failed to update container: %s doesn't exist", id))
		}
		ctx.Status(fiber.StatusOK)
		return nil
	}
}
func (c containerHandler) Delete() fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		id := ctx.Params("id")
		existing, err := c.controller.Get(id)
		if err != nil {
			return ctx.Status(fiber.StatusInternalServerError).SendString(err.Error())
		} 
		if existing == nil {
			return ctx.Status(fiber.StatusNotFound).SendString(fmt.Sprintf("failed to delete container: %s doesn't exist", id))
		}
		err = c.controller.Delete(id)
		if err != nil {
			return ctx.Status(fiber.StatusInternalServerError).SendString(err.Error())
		}
		ctx.Status(fiber.StatusOK)
		return nil
	}
}
func (c containerHandler) Start() fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		id := ctx.Params("id")
		existing, err := c.controller.Get(id)
		if err != nil {
			return ctx.Status(fiber.StatusInternalServerError).SendString(err.Error())
		} 
		if existing == nil {
			return ctx.Status(fiber.StatusNotFound).SendString(fmt.Sprintf("failed to start container: %s doesn't exist", id))
		}
		err = c.controller.Start(id)
		if err != nil {
			return ctx.Status(fiber.StatusInternalServerError).SendString(err.Error())
		}
		ctx.Status(fiber.StatusOK)
		return nil
	}
}
func (c containerHandler) Stop() fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		id := ctx.Params("id")
		existing, err := c.controller.Get(id)
		if err != nil {
			return ctx.Status(fiber.StatusInternalServerError).SendString(err.Error())
		} 
		if existing == nil {
			return ctx.Status(fiber.StatusNotFound).SendString(fmt.Sprintf("failed to stop container: %s doesn't exist", id))
		}
		err = c.controller.Stop(id)
		if err != nil {
			return ctx.Status(fiber.StatusInternalServerError).SendString(err.Error())
		}
		ctx.Status(fiber.StatusOK)
		return nil
	}
}

func NewContainerHandler(controller *controller.ContainerController) ContainerHandler {
	return &containerHandler{controller}
}
