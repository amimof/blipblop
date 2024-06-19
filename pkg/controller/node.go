package controller

import (
	"context"
	"time"

	"github.com/amimof/blipblop/api/services/nodes/v1"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/amimof/blipblop/pkg/runtime"
)

type NodeController struct {
	client  *client.ClientSet
	runtime runtime.Runtime
	//interval int
}

func (n *NodeController) Run(ctx context.Context, stop <-chan struct{}) {
	go func() {
		var lastStatus bool
		for {
			select {
			case <-stop:
				return
			default:
				ok, err := n.runtime.IsServing(ctx)
				if err != nil {
					logrus.Printf("error checking if runtime is serving: %s", err.Error())
					if lastStatus {
						err := n.client.NodeV1().SetNodeReady(ctx, n.node.GetName(), false)
						if err != nil {
							logrus.Printf("error setting node ready status to false: %s", err)
							//lastStatus = false
						}
						//lastStatus = false
					}
				}
				if ok && !lastStatus {
					err := n.client.NodeV1().SetNodeReady(ctx, n.node.GetName(), true)
					if err != nil {
						logrus.Printf("error setting node ready status to true: %s", err.Error())
						//lastStatus = false
					}
					lastStatus = true
				}
				time.Sleep(time.Second * 5)
			}
		}
	}()
}

// Recouncile ensures that desired containers matches with containers
// in the runtime environment. It removes any containers that are not
// desired (missing from the server) and adds those missing from runtime.
// It is preferrably run early during startup of the controller.
func (n *NodeController) Recouncile(ctx context.Context) error {
	return nil
}

func NewNodeController(c *client.ClientSet, rt runtime.Runtime) *NodeController {
	m := &NodeController{
		client:  c,
		runtime: rt,
	}
	return m
}
