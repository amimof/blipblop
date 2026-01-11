// Package get provides ability to get resources from the server
package get

import (
	"github.com/amimof/voiyd/pkg/client"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var output string

func NewCmdGet() *cobra.Command {
	var cfg client.Config
	getCmd := &cobra.Command{
		Use:   "get",
		Short: "Get a resource",
		Long:  "Get a resource",
		Example: `
# Get all containers
voiydctl get containers

# Get a specific container
voiydctl get container prometheus

# Get all nodes
voiydctl get nodes
`,
		Args: cobra.ExactArgs(1),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.ReadInConfig(); err != nil {
				logrus.Fatalf("error reading config: %v", err)
			}
			if err := viper.Unmarshal(&cfg); err != nil {
				logrus.Fatalf("error decoding config into struct: %v", err)
			}
			if err := cfg.Validate(); err != nil {
				logrus.Fatalf("config validation error: %v", err)
			}
			return nil
		},
	}

	getCmd.PersistentFlags().StringVarP(&output, "output", "o", "json", "Output format")

	getCmd.AddCommand(NewCmdGetEvent(&cfg))
	getCmd.AddCommand(NewCmdGetNode(&cfg))
	getCmd.AddCommand(NewCmdGetTask(&cfg))
	getCmd.AddCommand(NewCmdGetContainerSet(&cfg))
	getCmd.AddCommand(NewCmdGetVolume(&cfg))
	getCmd.AddCommand(NewCmdGetLease(&cfg))

	return getCmd
}
