package cmd

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/amimof/blipblop/cmd/bbctl/delete"
	"github.com/amimof/blipblop/cmd/bbctl/get"
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

	// Verbosity
	verbosity string
	// Server
	server string
	// Insecure
	insecure bool

	tlsCACert  string
	tlsCert    string
	tlsCertKey string

	clientSet *client.ClientSet
)

func init() {
	// Setup flags
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

	// Setup flags
	rootCmd.PersistentFlags().StringVarP(&server, "server", "s", "http://localhost:5700", "Address of the API Server")
	rootCmd.PersistentFlags().StringVarP(&tlsCACert, "tls-ca-certificate", "", "", "CA Certificate file path")
	rootCmd.PersistentFlags().StringVarP(&tlsCert, "tls-certificate", "", "", "Certificate file path")
	rootCmd.PersistentFlags().StringVarP(&tlsCertKey, "tls-certificate-key", "", "", "Certificate key file path")
	rootCmd.PersistentFlags().BoolVarP(&insecure, "insecure", "i", false, "Skip TLS certificate verification")
	rootCmd.PersistentFlags().StringVarP(&verbosity, "v", "v", "info", "number for the log level verbosity (debug, info, warn, error, fatal, panic)")

	// Setup sub-commands
	rootCmd.AddCommand(run.NewCmdRun(clientSet))
	rootCmd.AddCommand(delete.NewCmdDelete(clientSet))
	rootCmd.AddCommand(get.NewCmdGet())
	rootCmd.AddCommand(stop.NewCmdStop(clientSet))
	rootCmd.AddCommand(run.NewCmdRun(clientSet))
	rootCmd.AddCommand(start.NewCmdStart(clientSet))

	return rootCmd
}
