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
	"time"
)

var containerController *ContainerController

type ContainerController struct {
	repo   repo.ContainerRepo
	client *client.LocalClient
}

func (c *ContainerController) Get(id string) (*models.Container, error) {
	ctx := context.Background()
	container, err := c.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return container, c.client.Publish(ctx, NewEventFor(id, events.EventType_ContainerGet))
}

func (c *ContainerController) All() ([]*models.Container, error) {
	ctx := context.Background()
	containers, err := c.repo.GetAll(ctx)
	if err != nil {
		return nil, err
	}
	return containers, c.client.Publish(ctx, NewEventFor("", events.EventType_ContainerGetAll))
}

func (c *ContainerController) Create(unit *models.Container) error {
	ctx := context.Background()
	unit.Created = time.Now()
	unit.Updated = time.Now()
	unit.Revision = 1
	err := c.repo.Set(ctx, unit)
	if err != nil {
		return err
	}
	return c.client.Publish(ctx, NewEventFor(*unit.Name, events.EventType_ContainerCreate))
}

func (c *ContainerController) Start(id string) error {
	ctx := context.Background()
	err := c.repo.Start(ctx, id)
	if err != nil {
		return err
	}
	return c.client.Publish(ctx, NewEventFor(id, events.EventType_ContainerStart))
}

func (c *ContainerController) Stop(id string) error {
	ctx := context.Background()
	err := c.repo.Stop(ctx, id)
	if err != nil && !strings.Contains(err.Error(), "process already finished") {
		return err
	}
	return c.client.Publish(ctx, NewEventFor(id, events.EventType_ContainerStop))
}

func (c *ContainerController) Delete(id string) error {
	ctx := context.Background()
	err := c.repo.Delete(ctx, id)
	if err != nil {
		return err
	}
	return c.client.Publish(ctx, NewEventFor(id, events.EventType_ContainerDelete))
}

func NewEventFor(id string, t events.EventType) *events.Event {
	return &events.Event{
		Type:      t,
		Id:        id,
		EventId:   uuid.New().String(),
		Timestamp: ptypes.TimestampNow(),
	}
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
