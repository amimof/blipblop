// Package stop provides ability to stop resources
package stop

import (
	"time"

	"github.com/amimof/blipblop/pkg/client"
	"github.com/spf13/cobra"
)

var (
	force       bool
	wait        bool
	waitTimeout time.Duration
)

func NewCmdStop(cfg *client.Config) *cobra.Command {
	stopCmd := &cobra.Command{
		Use:     "stop",
		Short:   "Stop a resource",
		Long:    "Stop a resource",
		Example: `bbctl stop container`,
		Args:    cobra.ExactArgs(1),
	}

	stopCmd.PersistentFlags().BoolVar(&force,
		"force",
		false,
		"Attempt forceful shutdown of the continaner",
	)
	stopCmd.PersistentFlags().BoolVarP(
		&wait,
		"wait",
		"w",
		false,
		"Wait for command to finish",
	)
	stopCmd.PersistentFlags().DurationVarP(
		&waitTimeout,
		"timeout",
		"",
		time.Second*30,
		"How long in seconds to wait for container to start before giving up",
	)
	stopCmd.AddCommand(NewCmdStopContainer(cfg))

	return stopCmd
}
