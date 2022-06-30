package handlers

import (
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
		return nil
	}
}
func (c containerHandler) Delete() fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		err := c.controller.Delete(ctx.Params("id"))
		if err != nil {
			return ctx.Status(fiber.StatusInternalServerError).SendString(err.Error())
		}
		ctx.Status(fiber.StatusOK)
		return nil
	}
}
func (c containerHandler) Start() fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		err := c.controller.Start(ctx.Params("id"))
		if err != nil {
			return ctx.Status(fiber.StatusInternalServerError).SendString(err.Error())
		}
		ctx.Status(fiber.StatusOK)
		return nil
	}
}
func (c containerHandler) Stop() fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		err := c.controller.Stop(ctx.Params("id"))
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
