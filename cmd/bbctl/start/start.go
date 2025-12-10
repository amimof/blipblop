// Package start provides ability to start resources
package start

import (
	"time"

	"github.com/amimof/blipblop/pkg/client"
	"github.com/spf13/cobra"
)

var (
	wait        bool
	waitTimeout time.Duration
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

	startCmd.PersistentFlags().BoolVarP(
		&wait,
		"wait",
		"w",
		false,
		"Wait for command to finish",
	)
	startCmd.PersistentFlags().DurationVarP(
		&waitTimeout,
		"timeout",
		"",
		time.Second*30,
		"How long in seconds to wait for container to start before giving up",
	)

	return startCmd
}
