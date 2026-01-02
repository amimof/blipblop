// Package edit provides ability to edit resources from the server
package edit

import (
	"github.com/amimof/blipblop/pkg/client"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	wait               bool
	waitTimeoutSeconds uint64
	resourceLabels     map[string]string
	output             string
)

var editCmd *cobra.Command

func NewCmdEdit() *cobra.Command {
	var cfg client.Config
	editCmd = &cobra.Command{
		Use:     "edit NAME",
		Short:   "Edit a resource",
		Long:    "Edit a resource",
		Example: `bbctl edit set`,
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

	editCmd.PersistentFlags().BoolVarP(&wait, "wait", "w", true, "Wait for command to finish")
	editCmd.PersistentFlags().Uint64VarP(&waitTimeoutSeconds, "timeout", "", 30, "How long in seconds to wait for container to start before giving up")
	editCmd.PersistentFlags().StringToStringVarP(&resourceLabels, "labels", "l", map[string]string{}, "Resource labels as key value pair")
	editCmd.PersistentFlags().StringVarP(&output, "output", "o", "json", "Output format")
	editCmd.AddCommand(NewCmdEditContainer(&cfg))

	return editCmd
}
