// Package start provides ability to start resources
package start

import (
	"github.com/amimof/blipblop/pkg/client"
	"github.com/spf13/cobra"
)

func NewCmdStart(cfg *client.Config) *cobra.Command {
	startCmd := &cobra.Command{
		Use:     "start",
		Short:   "Start a resource",
		Long:    "Start a resource",
		Example: `bbctl start container`,
		Args:    cobra.ExactArgs(1),
	}

	startCmd.AddCommand(NewCmdStartContainer(cfg))

	return startCmd
}
