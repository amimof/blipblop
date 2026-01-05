// Package start provides ability to start resources
package start

import (
	"time"

	"github.com/amimof/voiyd/pkg/client"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	wait        bool
	waitTimeout time.Duration
)

func NewCmdStart() *cobra.Command {
	var cfg client.Config
	startCmd := &cobra.Command{
		Use:     "start NAME",
		Short:   "Start a resource",
		Long:    "Start a resource",
		Example: `voiydctl start task`,
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

	startCmd.AddCommand(NewCmdStartTask(&cfg))

	startCmd.PersistentFlags().BoolVarP(
		&wait,
		"wait",
		"w",
		true,
		"Wait for command to finish",
	)
	startCmd.PersistentFlags().DurationVarP(
		&waitTimeout,
		"timeout",
		"",
		time.Second*30,
		"How long in seconds to wait for task to start before giving up",
	)

	return startCmd
}
