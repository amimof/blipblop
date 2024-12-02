package informer

import (
	"context"

	eventsv1 "github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/pkg/logger"
)

func WithNodeEventInformerLogger(l logger.Logger) NewNodeEventInformerOption {
	return func(s *nodeEventInformer) {
		s.logger = l
	}
}

type NewNodeEventInformerOption func(s *nodeEventInformer)

type NodeEventHandlerFuncs struct {
	OnCreate  ResourceEventHandlerFunc
	OnUpdate  ResourceEventHandlerFunc
	OnDelete  ResourceEventHandlerFunc
	OnJoin    ResourceEventHandlerFunc
	OnForget  ResourceEventHandlerFunc
	OnConnect ResourceEventHandlerFunc
}

type nodeEventInformer struct {
	handlers NodeEventHandlerFuncs
	logger   logger.Logger
}

func (i *nodeEventInformer) Run(ctx context.Context, eventChan <-chan *eventsv1.Event) {
	for e := range eventChan {
		switch e.Type {
		case eventsv1.EventType_NodeCreate:
			if i.handlers.OnCreate != nil {
				if err := i.handlers.OnCreate(ctx, e); err != nil {
					handleError(err, e, i.logger)
				}
			}
		case eventsv1.EventType_NodeUpdate:
			if i.handlers.OnUpdate != nil {
				if err := i.handlers.OnUpdate(ctx, e); err != nil {
					handleError(err, e, i.logger)
				}
			}
		case eventsv1.EventType_NodeDelete:
			if i.handlers.OnDelete != nil {
				if err := i.handlers.OnDelete(ctx, e); err != nil {
					handleError(err, e, i.logger)
				}
			}
		case eventsv1.EventType_NodeJoin:
			if i.handlers.OnJoin != nil {
				if err := i.handlers.OnJoin(ctx, e); err != nil {
					handleError(err, e, i.logger)
				}
			}
		case eventsv1.EventType_NodeForget:
			if i.handlers.OnForget != nil {
				if err := i.handlers.OnForget(ctx, e); err != nil {
					handleError(err, e, i.logger)
				}
			}
		case eventsv1.EventType_NodeConnect:
			if i.handlers.OnConnect != nil {
				if err := i.handlers.OnConnect(ctx, e); err != nil {
					handleError(err, e, i.logger)
				}
			}
		}
	}
}

func NewNodeEventInformer(h NodeEventHandlerFuncs) EventInformer {
	return &nodeEventInformer{handlers: h}
}
