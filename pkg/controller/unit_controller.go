package controller

import (
	//"os"
	"log"
	"context"
	"github.com/containerd/containerd"
	gocni "github.com/containerd/go-cni"
	"github.com/amimof/blipblop/internal/repo"
	"github.com/containerd/containerd/api/events"
)

type unitController struct {
	informer *Informer
	repo repo.UnitRepo
}

// 
func (u *unitController) exitHandler(e *events.TaskExit) {
	ctx := context.Background()
	err := u.repo.Kill(ctx, e.ID)
	if err != nil {
		log.Printf("Got an error while trying to kill task %s. The error was %s", e.ID, err.Error())
	}
	err = u.repo.Start(ctx, e.ContainerID)
	log.Printf("Task %s just exited", e.ID)
}

func (u *unitController) createHandler(e *events.TaskCreate) {
	log.Printf("Task was just created with PID %d", e.Pid)
}

func (u *unitController) Run(stop <-chan struct{}) {
	go u.informer.Watch(stop)
}

func NewUnitController(client *containerd.Client, cni gocni.CNI) Controller {
	c := &unitController{
		repo: repo.NewUnitRepo(client, cni),
	}
	informer := NewInformer(client)
	informer.AddHandler(&HandlerFuncs{
		OnTaskExit: c.exitHandler,
		OnTaskCreate: c.createHandler,
	})
	c.informer = informer
	return c
}

