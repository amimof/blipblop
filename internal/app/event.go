package app

import (
	"context"
	"sync"

	"github.com/amimof/voiyd/pkg/events"
	"github.com/amimof/voiyd/pkg/keys"
	"github.com/amimof/voiyd/pkg/logger"
	"github.com/amimof/voiyd/pkg/repository"

	eventsv1 "github.com/amimof/voiyd/api/services/events/v1"
)

type EventService struct {
	mu       sync.Mutex
	Repo     *repository.Repo[*eventsv1.Event]
	Logger   logger.Logger
	Exchange *events.Exchange
	Manager  SessionManager
}

func (n *EventService) Get(ctx context.Context, id keys.ID) (*eventsv1.Event, error) {
	ctx, span := tracer.Start(ctx, "event.Get")
	defer span.End()

	return n.Repo.Get(ctx, id)
}

func (n *EventService) Create(ctx context.Context, event *eventsv1.Event) (*eventsv1.Event, error) {
	ctx, span := tracer.Start(ctx, "event.Create")
	defer span.End()

	n.mu.Lock()
	defer n.mu.Unlock()

	return n.Repo.Create(ctx, event)
}

func (n *EventService) Delete(ctx context.Context, id keys.ID) error {
	ctx, span := tracer.Start(ctx, "event.Delete")
	defer span.End()

	n.mu.Lock()
	defer n.mu.Unlock()

	return n.Repo.Delete(ctx, id)
}

func (n *EventService) List(ctx context.Context, limit int32) ([]*eventsv1.Event, error) {
	ctx, span := tracer.Start(ctx, "event.List")
	defer span.End()

	return n.Repo.List(ctx, limit)
}

// Publish implements events.EventServiceClient.
func (n *EventService) Publish(ctx context.Context, ev *eventsv1.Event) (*eventsv1.Event, error) {
	ctx, span := tracer.Start(ctx, "event.Publish")
	defer span.End()

	res, err := n.Repo.Create(ctx, ev)
	if err != nil {
		return nil, err
	}

	err = n.Exchange.Publish(ctx, res)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// Subscribe implements events.EventServiceClient.
func (n *EventService) Subscribe(ctx context.Context, in NodeConnectInput) (Session, error) {
	return n.Manager.Connect(ctx, nil, in)
}
