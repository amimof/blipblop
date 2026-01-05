// Package cmd provides command line capabilities to build cli tool
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/amimof/voiyd/cmd/voiydctl/apply"
	"github.com/amimof/voiyd/cmd/voiydctl/config"
	"github.com/amimof/voiyd/cmd/voiydctl/create"
	"github.com/amimof/voiyd/cmd/voiydctl/delete"
	"github.com/amimof/voiyd/cmd/voiydctl/edit"
	"github.com/amimof/voiyd/cmd/voiydctl/get"
	"github.com/amimof/voiyd/cmd/voiydctl/log"
	"github.com/amimof/voiyd/cmd/voiydctl/run"
	"github.com/amimof/voiyd/cmd/voiydctl/start"
	"github.com/amimof/voiyd/cmd/voiydctl/stop"
	"github.com/amimof/voiyd/cmd/voiydctl/upgrade"
)

var (
	rootCmd = &cobra.Command{
		SilenceUsage:  true,
		SilenceErrors: true,
		Use:           "voiydctl",
		Short:         "Lightweight, event‑driven orchestration for container workloads",
		Long:          `voiydctl is a command line tool for interacting with voiyd-server.`,
	}
	configFile   string
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
	viper.SetConfigFile(configFile)
	viper.SetConfigType("yaml")
}

func SetVersionInfo(version, commit, date, branch, goversion string) {
	rootCmd.Version = fmt.Sprintf("Version:\t%s\nCommit:\t%v\nBuilt:\t%s\nBranch:\t%s\nGo Version:\t%s\n", version, commit, date, branch, goversion)
}

func NewDefaultCommand() *cobra.Command {
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		lvl, err := logrus.ParseLevel(verbosity)
		if err != nil {
			return err
		}
		logrus.SetLevel(lvl)
		return nil
	}

	// Figure out path to default config file
	home, err := os.UserHomeDir()
	if err != nil {
		logrus.Fatalf("home directory cannot be determined: %v", err)
	}
	defaultConfigPath := filepath.Join(home, ".voiyd", "voiydctl.yaml")

	// Setup flags
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "", defaultConfigPath, "config file")
	rootCmd.PersistentFlags().StringVarP(&server, "server", "s", "localhost:5700", "Address of the API Server")
	rootCmd.PersistentFlags().StringVarP(&tlsCACert, "tls-ca-certificate", "", "", "CA Certificate file path")
	rootCmd.PersistentFlags().StringVarP(&tlsCert, "tls-certificate", "", "", "Certificate file path")
	rootCmd.PersistentFlags().StringVarP(&tlsCertKey, "tls-certificate-key", "", "", "Certificate key file path")
	rootCmd.PersistentFlags().StringVarP(&otelEndpoint, "otel-endpoint", "", "", "Endpoint address of OpenTelemetry collector")
	rootCmd.PersistentFlags().BoolVarP(&insecure, "insecure", "i", false, "Skip TLS certificate verification")
	rootCmd.PersistentFlags().StringVarP(&verbosity, "v", "v", "info", "number for the log level verbosity (debug, info, warn, error, fatal, panic)")

	// Setup sub-commands
	rootCmd.AddCommand(run.NewCmdRun())
	rootCmd.AddCommand(delete.NewCmdDelete())
	rootCmd.AddCommand(get.NewCmdGet())
	rootCmd.AddCommand(stop.NewCmdStop())
	rootCmd.AddCommand(run.NewCmdRun())
	rootCmd.AddCommand(start.NewCmdStart())
	rootCmd.AddCommand(create.NewCmdCreate())
	rootCmd.AddCommand(edit.NewCmdEdit())
	rootCmd.AddCommand(log.NewCmdLog())
	rootCmd.AddCommand(apply.NewCmdApply())
	rootCmd.AddCommand(upgrade.NewCmdUpgrade())
	rootCmd.AddCommand(config.NewCmdConfig())

	return rootCmd
}
