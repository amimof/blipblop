// Package stop provides ability to stop resources
package stop

import (
	"github.com/amimof/blipblop/pkg/client"
	"github.com/spf13/cobra"
)

var force bool

func NewCmdStop(cfg *client.Config) *cobra.Command {
	stopCmd := &cobra.Command{
		Use:     "stop",
		Short:   "Stop a resource",
		Long:    "Stop a resource",
		Example: `bbctl stop container`,
		Args:    cobra.ExactArgs(1),
	}

	stopCmd.PersistentFlags().BoolVar(&force, "force", false, "Attempt forceful shutdown of the continaner")

	stopCmd.AddCommand(NewCmdStopContainer(cfg))

	return stopCmd
}
