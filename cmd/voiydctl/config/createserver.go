package config

import (
	"os"

	"github.com/amimof/voiyd/pkg/client"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

func NewCmdConfigCreateServer() *cobra.Command {
	var cfg client.Config
	cmd := &cobra.Command{
		Use:   "create-server NAME",
		Short: "Add a server to voiydctl client configuration",
		Long:  "Add a server to voiydctl client configuration",
		Example: `
# Create a server with TLS
voiydctl config create-server dev --address localhost:5743 --ca ca.crt
`,
		Args: cobra.ExactArgs(1),
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
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			serverName := args[0]

			newServer := &client.Server{
				Name:    serverName,
				Address: address,
			}

			if tls {
				newServer.TLSConfig = &client.TLSConfig{
					Insecure: insecure,
				}
				if caFile != "" {
					caData, err := os.ReadFile(caFile)
					if err != nil {
						logrus.Fatalf("error reading ca file: %v", err)
					}
					newServer.TLSConfig.CA = string(caData)
				}

				if certFile != "" {
					certData, err := os.ReadFile(certFile)
					if err != nil {
						logrus.Fatalf("error reading certificate file: %v", err)
					}
					newServer.TLSConfig.Certificate = string(certData)
				}

				if keyFile != "" {
					keyData, err := os.ReadFile(keyFile)
					if err != nil {
						logrus.Fatalf("error reading key file: %v", err)
					}
					newServer.TLSConfig.Key = string(keyData)
				}
			}

			if current {
				cfg.Current = newServer.Name
			}

			err := cfg.AddServer(newServer)
			if err != nil {
				logrus.Fatalf("error addding server to config: %v", err)
			}

			b, err := yaml.Marshal(cfg)
			if err != nil {
				logrus.Fatalf("error marshal: %v", err)
			}

			err = os.WriteFile(viper.GetViper().ConfigFileUsed(), b, 0o666)
			if err != nil {
				logrus.Fatalf("error writing config file: %v", err)
			}

			logrus.Infof("Added server %v to configuration", serverName)
		},
	}

	return cmd
}
