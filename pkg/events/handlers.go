package events

import (
	"context"

	"github.com/amimof/voiyd/pkg/logger"

	eventsv1 "github.com/amimof/voiyd/api/services/events/v1"
	leasesv1 "github.com/amimof/voiyd/api/services/leases/v1"
	nodesv1 "github.com/amimof/voiyd/api/services/nodes/v1"
	tasksv1 "github.com/amimof/voiyd/api/services/tasks/v1"
	volumesv1 "github.com/amimof/voiyd/api/services/volumes/v1"
)

type (
	HandlerFunc       func(context.Context, *eventsv1.Event) error
	TaskHandlerFunc   func(context.Context, *tasksv1.Task) error
	VolumeHandlerFunc func(context.Context, *volumesv1.Volume) error
	NodeHandlerFunc   func(context.Context, *nodesv1.Node) error
	LeaseHandlerFunc  func(context.Context, *leasesv1.Lease) error
)

func HandleErrors(log logger.Logger, h HandlerFunc) HandlerFunc {
	return func(ctx context.Context, ev *eventsv1.Event) error {
		err := h(ctx, ev)
		if err != nil {
			log.Error("handler returned error", "error", err, "event", ev.GetType().String())
			return err
		}
		return nil
	}
}

func Handle(h HandlerFunc) HandlerFunc {
	return func(ctx context.Context, ev *eventsv1.Event) error {
		return h(ctx, ev)
	}
}

func HandleTask(h TaskHandlerFunc) HandlerFunc {
	return func(ctx context.Context, ev *eventsv1.Event) error {
		var task tasksv1.Task
		err := ev.GetObject().UnmarshalTo(&task)
		if err != nil {
			return err
		}
		return h(ctx, &task)
	}
}

func HandleVolume(h VolumeHandlerFunc) HandlerFunc {
	return func(ctx context.Context, ev *eventsv1.Event) error {
		var volume volumesv1.Volume
		err := ev.GetObject().UnmarshalTo(&volume)
		if err != nil {
			return err
		}
		return h(ctx, &volume)
	}
}

func HandleNode(h NodeHandlerFunc) HandlerFunc {
	return func(ctx context.Context, ev *eventsv1.Event) error {
		var node nodesv1.Node
		err := ev.GetObject().UnmarshalTo(&node)
		if err != nil {
			return err
		}
		return h(ctx, &node)
	}
}

func HandleLease(h LeaseHandlerFunc) HandlerFunc {
	return func(ctx context.Context, ev *eventsv1.Event) error {
		var lease leasesv1.Lease
		err := ev.GetObject().UnmarshalTo(&lease)
		if err != nil {
			return err
		}
		return h(ctx, &lease)
	}
}
