package config

import (
	"fmt"
	"os"

	"github.com/ghodss/yaml"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/amimof/voiyd/pkg/client"
)

func NewCmdConfigView() *cobra.Command {
	var cfg client.Config
	cmd := &cobra.Command{
		Use:   "view",
		Short: "View the entire client configuration",
		Long:  "View the entire client configuration",
		Args:  cobra.MaximumNArgs(0),
		PreRunE: func(cmd *cobra.Command, args []string) error {
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
		Run: func(cmd *cobra.Command, args []string) {
			b, err := yaml.Marshal(cfg)
			if err != nil {
				logrus.Fatalf("error marshal: %v", err)
			}

			err = os.WriteFile(viper.GetViper().ConfigFileUsed(), b, 0o666)
			if err != nil {
				logrus.Fatalf("error writing config file: %v", err)
			}

			fmt.Println(string(b))
		},
	}

	return cmd
}
