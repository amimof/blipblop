// Package create provides ability to create resources from the server
package create

import (
	"github.com/amimof/voiyd/cmd/voiydctl/create/volume"
	"github.com/amimof/voiyd/pkg/client"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	wait               bool
	waitTimeoutSeconds uint64
	resourceLabels     map[string]string
)

var createCmd *cobra.Command

func NewCmdCreate() *cobra.Command {
	var cfg client.Config
	createCmd = &cobra.Command{
		Use:     "create",
		Short:   "Create a resource",
		Long:    "Create a resource",
		Example: `voiydctl create set`,
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

	createCmd.PersistentFlags().BoolVarP(&wait, "wait", "w", true, "Wait for command to finish")
	createCmd.PersistentFlags().Uint64VarP(&waitTimeoutSeconds, "timeout", "", 30, "How long in seconds to wait for container to start before giving up")
	createCmd.PersistentFlags().StringToStringVarP(&resourceLabels, "labels", "l", map[string]string{}, "Resource labels as key value pair")
	createCmd.AddCommand(NewCmdCreateSet(&cfg))
	createCmd.AddCommand(volume.NewCmdCreateVolume(&cfg))

	return createCmd
}
