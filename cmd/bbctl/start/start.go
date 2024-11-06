package start

import (
	"github.com/amimof/blipblop/pkg/client"
	"github.com/spf13/cobra"
)

var (
	wait               bool
	waitTimeoutSeconds uint64
)

func NewCmdStart(cfg *client.Config) *cobra.Command {
	startCmd := &cobra.Command{
		Use:     "start",
		Short:   "Start a resource",
		Long:    "Start a resource",
		Example: `bbctl start container`,
		Args:    cobra.ExactArgs(1),
	}

	startCmd.PersistentFlags().BoolVarP(
		&wait,
		"wait",
		"w",
		false,
		"Wait for command to finish",
	)
	startCmd.PersistentFlags().Uint64VarP(
		&waitTimeoutSeconds,
		"timeout",
		"",
		30,
		"How long in seconds to wait for container to start before giving up",
	)

	startCmd.AddCommand(NewCmdStartContainer(cfg))

	return startCmd
}
