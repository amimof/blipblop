package delete

import (
	"github.com/amimof/blipblop/pkg/client"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewCmdDelete() *cobra.Command {
	var cfg client.Config
	deleteCmd := &cobra.Command{
		Use:     "delete",
		Short:   "Delete a resource",
		Long:    "Delete a resource",
		Example: `bbctl delete container`,
		Args:    cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
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

	deleteCmd.AddCommand(NewCmdDeleteContainer(&cfg))
	deleteCmd.AddCommand(NewCmdDeleteNode(&cfg))
	deleteCmd.AddCommand(NewCmdDeleteContainerSet(&cfg))
	deleteCmd.AddCommand(NewCmdDeleteVolume(&cfg))

	return deleteCmd
}
