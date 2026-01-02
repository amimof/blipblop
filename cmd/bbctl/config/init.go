package config

import (
	"errors"
	"fmt"
	"os"
	"path"

	"github.com/ghodss/yaml"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/amimof/blipblop/pkg/client"
)

func NewCmdConfigInit() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize client configuration",
		Long:  "Initialize client configuration",
		Example: `
# Init client configuration in default location 
bbctl config init

# Init client configuration in alternate location 
bbctl config init --config /etc/blipblop/bbctl.yaml
`,
		Args: cobra.MaximumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				return err
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			configPath := viper.ConfigFileUsed()

			if configExists(configPath) {
				logrus.Fatalf("config already exists at %s", configPath)
			}

			cfg := &client.Config{
				Version: "config/v1",
				Servers: []*client.Server{},
			}

			b, err := yaml.Marshal(cfg)
			if err != nil {
				logrus.Fatalf("error marshal: %v", err)
			}

			err = os.MkdirAll(path.Dir(configPath), 0o755)
			if err != nil {
				logrus.Fatalf("error creating config dir: %v", err)
			}

			err = os.WriteFile(configPath, b, 0o666)
			if err != nil {
				logrus.Fatalf("error writing config file: %v", err)
			}

			if err := viper.ReadInConfig(); err != nil {
				logrus.Fatalf("error reading config: %v", err)
			}

			fmt.Printf("Configuration created in %s\n", configPath)
		},
	}

	return cmd
}

func configExists(p string) bool {
	_, err := os.Stat(p)
	return !errors.Is(err, os.ErrNotExist)
}
