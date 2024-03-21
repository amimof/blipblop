package get

import (
	"context"
	"fmt"
	"log"

	"github.com/amimof/blipblop/pkg/client"
	"github.com/spf13/cobra"
)

func NewCmdGetNode(c *client.ClientSet) *cobra.Command {
	runCmd := &cobra.Command{
		Use:     "node",
		Short:   "Get a nodes",
		Long:    "Get a nodes",
		Example: `bbctl get nodes`,
		Args:    cobra.ArbitraryArgs,
		Run: func(_ *cobra.Command, args []string) {
			nodes, err := c.NodeV1().ListNodes(context.Background())
			if err != nil {
				log.Fatal(err)
			}
			fmt.Printf("%s\t%s\t%s\n", "NAME", "REVISION", "READY")
			for _, n := range nodes {
				fmt.Printf("%s\t%d\t%t\n", n.GetName(), n.GetRevision(), n.GetStatus().GetReady())
			}
		},
	}

	return runCmd
}
