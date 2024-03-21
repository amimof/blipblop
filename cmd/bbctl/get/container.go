package get

import (
	"context"
	"fmt"
	"log"

	"github.com/amimof/blipblop/pkg/client"
	"github.com/spf13/cobra"
)

func NewCmdGetContainer(c *client.ClientSet) *cobra.Command {
	runCmd := &cobra.Command{
		Use:     "container",
		Short:   "Get a container",
		Long:    "Get a container",
		Example: `bbctl get container`,
		Args:    cobra.ArbitraryArgs,
		Run: func(_ *cobra.Command, args []string) {
			containers, err := c.ContainerV1().ListContainers(context.Background())
			if err != nil {
				log.Fatal(err)
			}
			fmt.Printf("%s\t%s\t%s\t%s\n", "NAME", "REVISION", "STATE", "NODE")
			for _, c := range containers {
				fmt.Printf("%s\t%d\t%s\t%s\n", c.GetName(), c.GetRevision(), c.GetStatus().GetState(), c.GetStatus().GetNode())
			}
		},
	}

	return runCmd
}
