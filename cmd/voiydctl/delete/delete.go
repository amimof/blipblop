package delete

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

func NewCmdDelete() *cobra.Command {
	var cfg client.Config
	deleteCmd := &cobra.Command{
		Use:     "delete",
		Short:   "Delete a resource",
		Long:    "Delete a resource",
		Example: `voiydctl delete container`,
		Args:    cobra.ExactArgs(1),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
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

	deleteCmd.AddCommand(NewCmdDeleteTask(&cfg))
	deleteCmd.AddCommand(NewCmdDeleteNode(&cfg))
	deleteCmd.AddCommand(NewCmdDeleteContainerSet(&cfg))
	deleteCmd.AddCommand(NewCmdDeleteVolume(&cfg))
	deleteCmd.AddCommand(NewCmdDeleteLease(&cfg))

	deleteCmd.PersistentFlags().BoolVar(&force,
		"force",
		false,
		"Attempt forceful shutdown of the task",
	)
	deleteCmd.PersistentFlags().BoolVarP(
		&wait,
		"wait",
		"w",
		true,
		"Wait for command to finish",
	)
	deleteCmd.PersistentFlags().DurationVarP(
		&waitTimeout,
		"timeout",
		"",
		time.Second*30,
		"How long in seconds to wait before giving up",
	)

	return deleteCmd
}
