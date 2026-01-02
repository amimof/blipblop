// Package config provides ability to manage bbctl configuration
package config

import (
	"github.com/spf13/cobra"
)

var (
	tls      bool
	insecure bool
	current  bool
	caFile   string
	certFile string
	keyFile  string
	address  string
)

func NewCmdConfig() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "config",
		Short:   "Manage bbctl client configuration",
		Long:    "Manage bbctl client configuration",
		Example: ``,
		Args:    cobra.ExactArgs(1),
	}

	cmd.PersistentFlags().BoolVar(&insecure, "insecure", false, "Skip TLS verification. Not recommended")
	cmd.PersistentFlags().BoolVar(&current, "current", true, "Set as current server")
	cmd.PersistentFlags().BoolVar(&tls, "tls", false, "Use TLS for this server")
	cmd.PersistentFlags().StringVar(&address, "address", "", "Endpoint address of the server")
	cmd.PersistentFlags().StringVar(&caFile, "ca", "", "Path to ca certificate file")
	cmd.PersistentFlags().StringVar(&certFile, "certificate", "", "Path to certificate file")
	cmd.PersistentFlags().StringVar(&keyFile, "key", "", "Path to private key file")

	cmd.AddCommand(NewCmdConfigCreateServer())
	cmd.AddCommand(NewCmdConfigUse())
	cmd.AddCommand(NewCmdConfigView())
	cmd.AddCommand(NewCmdConfigListServers())

	return cmd
}
