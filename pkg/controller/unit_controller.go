package controller

import (
	//"os"
	"context"
	"github.com/amimof/blipblop/internal/repo"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/api/events"
	gocni "github.com/containerd/go-cni"
	"log"
)

type unitController struct {
	informer *Informer
	repo     repo.UnitRepo
	client   *client.Client
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

func NewUnitController(client *client.Client, cclient *containerd.Client, cni gocni.CNI) Controller {
	c := &unitController{
		client: client,
		repo:   repo.NewUnitRepo(cclient, cni),
	}
	informer := NewInformer(cclient)
	informer.AddHandler(&HandlerFuncs{
		OnTaskExit:   c.exitHandler,
		OnTaskCreate: c.createHandler,
	})
	c.informer = informer
	return c
}
