package cmd

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/amimof/blipblop/cmd/bbctl/create"
	"github.com/amimof/blipblop/cmd/bbctl/delete"
	"github.com/amimof/blipblop/cmd/bbctl/edit"
	"github.com/amimof/blipblop/cmd/bbctl/get"
	"github.com/amimof/blipblop/cmd/bbctl/log"
	"github.com/amimof/blipblop/cmd/bbctl/run"
	"github.com/amimof/blipblop/cmd/bbctl/start"
	"github.com/amimof/blipblop/cmd/bbctl/stop"
	"github.com/amimof/blipblop/pkg/client"
)

var (
	rootCmd = &cobra.Command{
		SilenceUsage:  true,
		SilenceErrors: true,
		Use:           "bbctl",
		Short:         "Distributed containerd workloads",
		Long:          `bbctl is a command line tool for interacting with blipblop-server.`,
	}
	config       string
	verbosity    string
	server       string
	insecure     bool
	tlsCACert    string
	tlsCert      string
	tlsCertKey   string
	otelEndpoint string
)

func init() {
	cobra.OnInitialize(initConfig)
}

func initConfig() {
	if config != "" {
		viper.SetConfigFile(config)
		return
	}

	viper.SetConfigName("bbctl")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("/etc/blipblop")

	if err := viper.ReadInConfig(); err != nil {
		logrus.Fatalf("error reading config file %v", err)
	}
}

func SetVersionInfo(version, commit, date, branch, goversion string) {
	rootCmd.Version = fmt.Sprintf("Version:\t%s\nCommit:\t%v\nBuilt:\t%s\nBranch:\t%s\nGo Version:\t%s\n", version, commit, date, branch, goversion)
}

func NewDefaultCommand() *cobra.Command {
	var cfg client.Config
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		lvl, err := logrus.ParseLevel(verbosity)
		if err != nil {
			return err
		}
		logrus.SetLevel(lvl)
		if err := viper.Unmarshal(&cfg); err != nil {
			logrus.Fatalf("error decoding config into struct: %v", err)
		}

		if err := cfg.Validate(); err != nil {
			logrus.Fatalf("Config validation failed %v:", err)
		}
		return nil
	}

	// Setup flags
	rootCmd.PersistentFlags().StringVarP(&config, "config", "", "", "config file")
	rootCmd.PersistentFlags().StringVarP(&server, "server", "s", "localhost:5700", "Address of the API Server")
	rootCmd.PersistentFlags().StringVarP(&tlsCACert, "tls-ca-certificate", "", "", "CA Certificate file path")
	rootCmd.PersistentFlags().StringVarP(&tlsCert, "tls-certificate", "", "", "Certificate file path")
	rootCmd.PersistentFlags().StringVarP(&tlsCertKey, "tls-certificate-key", "", "", "Certificate key file path")
	rootCmd.PersistentFlags().StringVarP(&otelEndpoint, "otel-endpoint", "", "", "Endpoint address of OpenTelemetry collector")
	rootCmd.PersistentFlags().BoolVarP(&insecure, "insecure", "i", false, "Skip TLS certificate verification")
	rootCmd.PersistentFlags().StringVarP(&verbosity, "v", "v", "info", "number for the log level verbosity (debug, info, warn, error, fatal, panic)")

	// Setup sub-commands
	rootCmd.AddCommand(run.NewCmdRun(&cfg))
	rootCmd.AddCommand(delete.NewCmdDelete(&cfg))
	rootCmd.AddCommand(get.NewCmdGet(&cfg))
	rootCmd.AddCommand(stop.NewCmdStop(&cfg))
	rootCmd.AddCommand(run.NewCmdRun(&cfg))
	rootCmd.AddCommand(start.NewCmdStart(&cfg))
	rootCmd.AddCommand(create.NewCmdCreate(&cfg))
	rootCmd.AddCommand(edit.NewCmdEdit(&cfg))
	rootCmd.AddCommand(log.NewCmdLog(&cfg))

	return rootCmd
}
