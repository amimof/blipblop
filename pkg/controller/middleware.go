package controller

import (
	"context"
)

type Controller interface {
	Run(context.Context, <-chan struct{})
}

type ControllerManager struct {
	controllers []Controller
	stop        chan struct{}
}

func (c *ControllerManager) Register(m Controller) {
	if m != nil {
		c.controllers = append(c.controllers, m)
	}
}

func (c *ControllerManager) Run(ctx context.Context) {
	c.stop = make(chan struct{})
	for _, ctrl := range c.controllers {
		go ctrl.Run(ctx, c.stop)
	}
}

func (c *ControllerManager) Stop() {
	c.stop <- struct{}{}
}

func NewManager(c ...Controller) *ControllerManager {
	cm := &ControllerManager{}
	for _, ctrl := range c {
		cm.Register(ctrl)
	}
	return cm
}
