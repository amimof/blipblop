package stop

import (
	"context"
	"log"

	"github.com/amimof/blipblop/pkg/client"
	"github.com/spf13/cobra"
)

func NewCmdStopContainer(c *client.ClientSet) *cobra.Command {
	runCmd := &cobra.Command{
		Use:     "container",
		Short:   "Stop a container",
		Long:    "Stop a container",
		Example: `bbctl stop container NAME`,
		Args:    cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			ctx := context.Background()
			cname := args[0]
			ctr, err := c.ContainerV1().GetContainer(ctx, cname)
			if err != nil {
				log.Fatal(err)
			}
			log.Println(ctr)
			err = c.ContainerV1().KillContainer(context.Background(), ctr.Name)
			if err != nil {
				log.Fatal(err)
			}
		},
	}

	return runCmd
}
