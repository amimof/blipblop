package get

import (
	"context"
	"fmt"
	"log"
	"os"
	"text/tabwriter"
	"time"

	"github.com/amimof/blipblop/pkg/client"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewCmdGetNode() *cobra.Command {
	runCmd := &cobra.Command{
		Use:     "node",
		Short:   "Get a nodes",
		Long:    "Get a nodes",
		Example: `bbctl get nodes`,
		Args:    cobra.ArbitraryArgs,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				return err
			}
			return nil
		},
		Run: func(_ *cobra.Command, args []string) {
			server := viper.GetString("server")
			ctx := context.Background()

			// Setup writer
			wr := tabwriter.NewWriter(os.Stdout, 8, 8, 8, '\t', tabwriter.AlignRight)

			// Setup our client
			c, err := client.New(server)
			if err != nil {
				logrus.Fatal(err)
			}
			nodes, err := c.NodeV1().ListNodes(ctx)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Fprintf(wr, "%s\t%s\t%s\t%s\n", "NAME", "REVISION", "READY", "AGE")
			for _, n := range nodes {
				fmt.Fprintf(wr, "%s\t%d\t%t\t%s\n", n.GetName(), n.GetRevision(), n.GetStatus().GetReady(), time.Since(n.Created.AsTime()).Round(1*time.Second))
			}

			wr.Flush()
		},
	}

	return runCmd
}
