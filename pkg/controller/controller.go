package controller

import (
	"log"
)

type Controller interface {
	Run(<-chan struct{})
}

type ControllerManager struct {
	controllers []Controller
}

func (c *ControllerManager) Register(controller Controller) {
	if controller != nil {
		c.controllers = append(c.controllers, controller)
	}
}

func (c *ControllerManager) SpawnAll() {
	stop := make(chan struct{})
	for _, ctrl := range c.controllers {
		log.Println("Running ctrl...")
		go ctrl.Run(stop)
	}
}

func NewControllerManager(controller ...Controller) *ControllerManager {
	cm := &ControllerManager{}
	for _, ctrl := range controller {
		cm.Register(ctrl)
	}
	return cm
}