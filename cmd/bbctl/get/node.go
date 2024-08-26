package get

import (
	"context"
	"fmt"
	"log"
	"os"
	"text/tabwriter"

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
			wr := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', tabwriter.AlignRight)

			// Setup our client
			c, err := client.New(ctx, server)
			if err != nil {
				logrus.Fatal(err)
			}
			nodes, err := c.NodeV1().ListNodes(context.Background())
			if err != nil {
				log.Fatal(err)
			}
			fmt.Fprintln(wr, fmt.Sprintf("%s\t%s\t%s\t", "NAME", "REVISION", "READY"))
			for _, n := range nodes {
				fmt.Fprintln(wr, fmt.Sprintf("%s\t%d\t%t\t", n.GetName(), n.GetRevision(), n.GetStatus().GetReady()))
			}

			wr.Flush()
		},
	}

	return runCmd
}
