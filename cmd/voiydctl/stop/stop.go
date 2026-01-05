// Package stop provides ability to stop resources
package stop

import (
	"time"

	"github.com/amimof/voiyd/pkg/client"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	force       bool
	wait        bool
	waitTimeout time.Duration
)

func NewCmdStop() *cobra.Command {
	var cfg client.Config
	stopCmd := &cobra.Command{
		Use:     "stop",
		Short:   "Stop a resource",
		Long:    "Stop a resource",
		Example: `voiydctl stop task`,
		Args:    cobra.ExactArgs(1),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				return err
			}
			if err := viper.ReadInConfig(); err != nil {
				logrus.Fatalf("error reading config: %v", err)
			}
			if err := viper.Unmarshal(&cfg); err != nil {
				logrus.Fatalf("error decoding config into struct: %v", err)
			}
			if err := cfg.Validate(); err != nil {
				logrus.Fatal(err)
			}
			return nil
		},
	}

	stopCmd.PersistentFlags().BoolVar(&force,
		"force",
		false,
		"Attempt forceful shutdown of the task",
	)
	stopCmd.PersistentFlags().BoolVarP(
		&wait,
		"wait",
		"w",
		true,
		"Wait for command to finish",
	)
	stopCmd.PersistentFlags().DurationVarP(
		&waitTimeout,
		"timeout",
		"",
		time.Second*30,
		"How long in seconds to wait for task to start before giving up",
	)
	stopCmd.AddCommand(NewCmdStopTask(&cfg))

	return stopCmd
}
