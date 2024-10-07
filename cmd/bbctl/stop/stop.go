package stop

import (
	"github.com/amimof/blipblop/pkg/client"
	"github.com/spf13/cobra"
)

var (
	wait               bool
	waitTimeoutSeconds uint64
)

func NewCmdStop(cfg *client.Config) *cobra.Command {
	stopCmd := &cobra.Command{
		Use:     "stop",
		Short:   "Stop a resource",
		Long:    "Stop a resource",
		Example: `bbctl stop container`,
		Args:    cobra.ExactArgs(1),
	}

	stopCmd.PersistentFlags().BoolVarP(
		&wait,
		"wait",
		"w",
		true,
		"Wait for command to finish",
	)
	stopCmd.PersistentFlags().Uint64VarP(
		&waitTimeoutSeconds,
		"timeout",
		"",
		30,
		"How long in seconds to wait for container to stop before giving up",
	)

	stopCmd.AddCommand(NewCmdStopContainer(cfg))

	return stopCmd
}
