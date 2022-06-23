package handlers

import (
	"github.com/amimof/blipblop/internal/models"
	"github.com/amimof/blipblop/internal/services"
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
	svc *services.ContainerService
}

func (c containerHandler) Get() fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		unit, err := c.svc.Get(ctx.Params("id"))
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
		units, err := c.svc.All()
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
		err = c.svc.Create(unit)
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
		err := c.svc.Delete(ctx.Params("id"))
		if err != nil {
			return ctx.Status(fiber.StatusInternalServerError).SendString(err.Error())
		}
		ctx.Status(fiber.StatusOK)
		return nil
	}
}
func (c containerHandler) Start() fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		err := c.svc.Start(ctx.Params("id"))
		if err != nil {
			return ctx.Status(fiber.StatusInternalServerError).SendString(err.Error())
		}
		ctx.Status(fiber.StatusOK)
		return nil
	}
}
func (c containerHandler) Stop() fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		err := c.svc.Stop(ctx.Params("id"))
		if err != nil {
			return ctx.Status(fiber.StatusInternalServerError).SendString(err.Error())
		}
		ctx.Status(fiber.StatusOK)
		return nil
	}
}

func NewContainerHandler(svc *services.ContainerService) ContainerHandler {
	return &containerHandler{svc}
}
