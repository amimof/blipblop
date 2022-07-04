package controller

import (
	"context"
	"github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/internal/models"
	"github.com/amimof/blipblop/internal/repo"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/golang/protobuf/ptypes"
	"github.com/google/uuid"
	"strings"
)

var containerController *ContainerController

type ContainerController struct {
	repo   repo.ContainerRepo
	client *client.LocalClient
}

func (c *ContainerController) Get(id string) (*models.Container, error) {
	return c.repo.Get(context.Background(), id)
}

func (c *ContainerController) All() ([]*models.Container, error) {
	return c.repo.GetAll(context.Background())
}

func (c *ContainerController) Create(unit *models.Container) error {
	ctx := context.Background()
	err := c.repo.Set(ctx, unit)
	if err != nil {
		return err
	}
	e := &events.Event{
		Type:      events.EventType_ContainerCreate,
		Id:        *unit.Name,
		EventId:   uuid.New().String(),
		Timestamp: ptypes.TimestampNow(),
	}
	err = c.client.Publish(ctx, e)
	if err != nil {
		return err
	}
	return nil
}

func (c *ContainerController) Start(id string) error {
	return c.repo.Start(context.Background(), id)
}

func (c *ContainerController) Stop(id string) error {
	err := c.repo.Stop(context.Background(), id)
	if err != nil && !strings.Contains(err.Error(), "process already finished") {
		return err
	}
	return nil
}

func (c *ContainerController) Delete(id string) error {
	return c.repo.Delete(context.Background(), id)
}

func newContainerController(client *client.LocalClient, r repo.ContainerRepo) *ContainerController {
	return &ContainerController{
		repo:   r,
		client: client,
	}
}

func NewContainerController(client *client.LocalClient, r repo.ContainerRepo) *ContainerController {
	if containerController == nil {
		containerController = newContainerController(client, repo.NewInMemContainerRepo())
	}
	return containerController
}
