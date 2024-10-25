package controller

import (
	"context"
	"log"
	"time"

	eventsv1 "github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/amimof/blipblop/pkg/events"
	"github.com/amimof/blipblop/pkg/logger"
)

type ContainerSetController struct {
	clientset *client.ClientSet
	logger    logger.Logger
}

type NewContainerSetControllerOption func(c *ContainerSetController)

func WithContainerSetLogger(l logger.Logger) NewContainerSetControllerOption {
	return func(c *ContainerSetController) {
		c.logger = l
	}
}

func (c *ContainerSetController) Run(ctx context.Context, stopCh <-chan struct{}) {
	// Setup channels
	evt := make(chan *eventsv1.Event, 10)
	errChan := make(chan error, 10)

	// Define handlers
	handlers := events.ContainerSetEventHandlerFuncs{
		OnCreate: func(e *eventsv1.Event) error {
			log.Println("set controller OnCreate")
			return nil
		},
		OnUpdate: func(e *eventsv1.Event) error {
			log.Println("set controller OnUpdate")
			return nil
		},
		OnDelete: func(e *eventsv1.Event) error {
			log.Println("set controller OnDelete")
			return nil
		},
	}

	// Run informer
	informer := events.NewContainerSetEventInformer(handlers)
	go informer.Run(evt)

	// Subscribe with retry
	for {
		select {
		case <-stopCh:
			c.logger.Info("done watching, stopping subscription")
			return
		default:
			if err := c.clientset.EventV1().Subscribe(ctx, evt, errChan); err != nil {
				c.logger.Error("error occured during subscribe", "error", err)
			}

			c.logger.Info("attempting to re-subscribe to event server")
			time.Sleep(5 * time.Second)
		}
	}
}

func (c *ContainerSetController) onContainerSetCreate(e *eventsv1.Event) error {
	c.logger.Info("Handler not implemented", e.Type.String())
	return nil
}

func NewContainerSetController(cs *client.ClientSet, opts ...NewContainerSetControllerOption) *ContainerSetController {
	eh := &ContainerSetController{
		clientset: cs,
		logger:    logger.ConsoleLogger{},
	}

	for _, opt := range opts {
		opt(eh)
	}

	return eh
}
