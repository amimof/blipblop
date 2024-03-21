package cmd

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/amimof/blipblop/pkg/client"

	"github.com/amimof/blipblop/cmd/bbctl/delete"
	"github.com/amimof/blipblop/cmd/bbctl/get"
	"github.com/amimof/blipblop/cmd/bbctl/run"
)

var rootCmd = &cobra.Command{
	SilenceUsage:  true,
	SilenceErrors: true,
	Use:           "bbctl",
	Short:         "Distributed containerd workloads",
	Long: `bbctl is a command line tool for interacting with blipblop-server.
`,
}
var v string

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
		&v,
		"v",
		"v",
		"info",
		"number for the log level verbosity (debug, info, warn, error, fatal, panic)")

	// Setup our client
	ctx := context.Background()
	c, err := client.New(ctx, "localhost:5700")
	if err != nil {
		logrus.Fatal(err)
	}

	// Setup sub-commands
	rootCmd.AddCommand(run.NewCmdRun(c))
	rootCmd.AddCommand(delete.NewCmdDelete(c))
	rootCmd.AddCommand(get.NewCmdGet(c))
	return rootCmd
}
