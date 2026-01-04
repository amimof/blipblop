package config

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/amimof/voiyd/pkg/client"
)

func NewCmdConfigListServers() *cobra.Command {
	var cfg client.Config
	cmd := &cobra.Command{
		Use:   "list-servers",
		Short: "List all servers in client configuration",
		Long:  "List all servers in client configuration",
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
			// Setup writer
			wr := tabwriter.NewWriter(os.Stdout, 8, 8, 8, '\t', tabwriter.AlignRight)

			curr, err := cfg.CurrentServer()
			if err != nil {
				logrus.Fatal(err)
			}

			_, _ = fmt.Fprintf(wr, "%s\t%s\t%s\t%s\n", "NAME", "CURRENT", "ADDRESS", "TLS")

			for _, s := range cfg.Servers {
				isCurrent := curr.Name == s.Name
				hasTLS := s.TLSConfig != nil
				_, _ = fmt.Fprintf(wr, "%s\t%t\t%s\t%t\n",
					s.Name,
					isCurrent,
					s.Address,
					hasTLS,
				)
			}

			_ = wr.Flush()
		},
	}

	return cmd
}
