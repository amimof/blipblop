package handlers

import (
<<<<<<< HEAD
	"github.com/gofiber/fiber/v2"
	"github.com/amimof/blipblop/internal/models"
	"github.com/amimof/blipblop/internal/services"
=======
	"github.com/amimof/blipblop/internal/models"
	"github.com/amimof/blipblop/internal/services"
	"github.com/gofiber/fiber/v2"
>>>>>>> 4483218 (Split server and node:)
)

type UnitHandler interface {
	Get() fiber.Handler
<<<<<<< HEAD
	GetAll() fiber.Handler	
=======
	GetAll() fiber.Handler
>>>>>>> 4483218 (Split server and node:)
	Create() fiber.Handler
	Update() fiber.Handler
	Delete() fiber.Handler
	Start() fiber.Handler
	Stop() fiber.Handler
}

type unitHandler struct {
	svc *services.UnitService
}

func (u unitHandler) Get() fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		unit, err := u.svc.Get(ctx.Params("id"))
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
func (u unitHandler) GetAll() fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		units, err := u.svc.All()
		if err != nil {
			return ctx.Status(fiber.StatusInternalServerError).SendString(err.Error())
		}
		ctx.JSON(units)
		return nil
	}
}
func (u unitHandler) Create() fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		unit := new(models.Unit)
		err := ctx.BodyParser(unit)
		if err != nil {
			return ctx.Status(fiber.StatusInternalServerError).SendString(err.Error())
		}
		err = u.svc.Create(unit)
		if err != nil {
			return ctx.Status(fiber.StatusInternalServerError).SendString(err.Error())
		}
		ctx.JSON(unit)
		return nil
	}
}
func (u unitHandler) Update() fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		return nil
	}
}
func (u unitHandler) Delete() fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		err := u.svc.Delete(ctx.Params("id"))
		if err != nil {
			return ctx.Status(fiber.StatusInternalServerError).SendString(err.Error())
		}
		ctx.Status(fiber.StatusOK)
		return nil
	}
}
func (u unitHandler) Start() fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		err := u.svc.Start(ctx.Params("id"))
		if err != nil {
			return ctx.Status(fiber.StatusInternalServerError).SendString(err.Error())
		}
		ctx.Status(fiber.StatusOK)
		return nil
	}
}
func (u unitHandler) Stop() fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		err := u.svc.Stop(ctx.Params("id"))
		if err != nil {
			return ctx.Status(fiber.StatusInternalServerError).SendString(err.Error())
		}
		ctx.Status(fiber.StatusOK)
		return nil
	}
}

func NewUnitHandler(svc *services.UnitService) UnitHandler {
	return &unitHandler{svc}
}
<<<<<<< HEAD

=======
>>>>>>> 4483218 (Split server and node:)
