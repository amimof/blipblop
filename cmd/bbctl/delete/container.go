package delete

import (
	"context"
	"log"

	"github.com/amimof/blipblop/pkg/client"
	"github.com/spf13/cobra"
)

func NewCmdDeleteContainer(c *client.ClientSet) *cobra.Command {
	runCmd := &cobra.Command{
		Use:     "container",
		Short:   "Delete a container",
		Long:    "Delete a container",
		Example: `bbctl delete container NAME`,
		Args:    cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			ctx := context.Background()
			cname := args[0]
			ctr, err := c.ContainerV1().GetContainer(ctx, cname)
			if err != nil {
				log.Fatal(err)
			}
			log.Println(ctr)
			err = c.ContainerV1().DeleteContainer(context.Background(), ctr.Name)
			if err != nil {
				log.Fatal(err)
			}
		},
	}

	return runCmd
}
