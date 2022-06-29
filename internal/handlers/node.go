package handlers

import (
	"github.com/amimof/blipblop/internal/models"
	"github.com/amimof/blipblop/internal/controller"
	"github.com/gofiber/fiber/v2"
)

type NodeHandler interface {
	Get() fiber.Handler
	GetAll() fiber.Handler
	Create() fiber.Handler
	Delete() fiber.Handler
}

type nodeHandler struct {
	controller *controller.NodeController
}

func (n nodeHandler) Get() fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		node, err := n.controller.Get(ctx.Params("id"))
		if err != nil {
			return ctx.Status(fiber.StatusInternalServerError).SendString(err.Error())
		}
		if node != nil {
			return ctx.JSON(node)
		}
		ctx.Status(fiber.StatusNotFound)
		return nil
	}
}
func (n nodeHandler) GetAll() fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		nodes, err := n.controller.All()
		if err != nil {
			return ctx.Status(fiber.StatusInternalServerError).SendString(err.Error())
		}
		ctx.JSON(nodes)
		return nil
	}
}

func (n nodeHandler) Create() fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		node := new(models.Node)
		err := ctx.BodyParser(node)
		if err != nil {
			return ctx.Status(fiber.StatusInternalServerError).SendString(err.Error())
		}
		err = n.controller.Create(node)
		if err != nil {
			return ctx.Status(fiber.StatusInternalServerError).SendString(err.Error())
		}
		ctx.JSON(node)
		return nil
	}
}

func (n nodeHandler) Delete() fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		err := n.controller.Delete(ctx.Params("id"))
		if err != nil {
			return ctx.Status(fiber.StatusInternalServerError).SendString(err.Error())
		}
		ctx.Status(fiber.StatusOK)
		return nil
	}
}

func NewNodeHandler(controller *controller.NodeController) NodeHandler {
	return &nodeHandler{controller}
}
