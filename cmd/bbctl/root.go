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
)

var rootCmd = &cobra.Command{
	SilenceUsage:  true,
	SilenceErrors: true,
	Use:           "bbctl",
	Short:         "Distributed containerd workloads",
	Long: `bbctl is a command line tool for interacting with blipblop-server.
`,
}

var (
	// Verbosity
	v string
	// Server
	s string
)

func SetVersionInfo(version, commit, date, branch, goversion string) {
	rootCmd.Version = fmt.Sprintf("Version:\t%s\nCommit:\t%v\nBuilt:\t%s\nBranch:\t%s\nGo Version:\t%s\n", version, commit, date, branch, goversion)
}

func NewDefaultCommand() *cobra.Command {
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		lvl, err := logrus.ParseLevel(v)
		if err != nil {
			return err
		}
		logrus.SetLevel(lvl)
		return nil
	}

	// Setup flags
	rootCmd.PersistentFlags().StringVarP(
		&s,
		"server",
		"s",
		"https://localhost:5700",
		"Address of the API Server",
	)
	rootCmd.PersistentFlags().StringVarP(
		&v,
		"v",
		"v",
		"info",
		"number for the log level verbosity (debug, info, warn, error, fatal, panic)")

	// Setup sub-commands
	rootCmd.AddCommand(run.NewCmdRun())
	rootCmd.AddCommand(delete.NewCmdDelete())
	rootCmd.AddCommand(get.NewCmdGet())
	rootCmd.AddCommand(stop.NewCmdStop())
	rootCmd.AddCommand(run.NewCmdRun())
	rootCmd.AddCommand(start.NewCmdStart())

	return rootCmd
}
