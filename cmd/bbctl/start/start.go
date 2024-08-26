package start

import (
	"github.com/spf13/cobra"
)

func NewCmdStart() *cobra.Command {
	startCmd := &cobra.Command{
		Use:     "start",
		Short:   "Start a resource",
		Long:    "Start a resource",
		Example: `bbctl start container`,
		Args:    cobra.ExactArgs(1),
	}

	startCmd.AddCommand(NewCmdStartContainer())

	return startCmd
}
