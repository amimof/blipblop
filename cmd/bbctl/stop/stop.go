package stop

import (
	"github.com/spf13/cobra"

	"github.com/amimof/blipblop/pkg/client"
)

func NewCmdStop(c *client.ClientSet) *cobra.Command {
	stopCmd := &cobra.Command{
		Use:     "stop",
		Short:   "Stop a resource",
		Long:    "Stop a resource",
		Example: `bbctl stop container`,
		Args:    cobra.ExactArgs(1),
	}

	stopCmd.AddCommand(NewCmdStopContainer(c))

	return stopCmd
}
