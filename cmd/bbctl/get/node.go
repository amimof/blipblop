package get

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/amimof/blipblop/pkg/client"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

func NewCmdGetNode() *cobra.Command {
	runCmd := &cobra.Command{
		Use:     "node",
		Short:   "Get a nodes",
		Long:    "Get a nodes",
		Example: `bbctl get nodes`,
		Args:    cobra.MaximumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				return err
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Setup client
			c, err := client.New(ctx, viper.GetString("server"), client.WithTLSConfigFromFlags(cmd.Flags()))
			if err != nil {
				logrus.Fatalf("error setting up client: %v", err)
			}

			// Setup writer
			wr := tabwriter.NewWriter(os.Stdout, 8, 8, 8, '\t', tabwriter.AlignRight)

			if len(args) == 1 {
				var b bytes.Buffer
				enc := yaml.NewEncoder(&b)
				enc.SetIndent(2)
				cname := args[0]
				node, err := c.NodeV1().Get(context.Background(), cname)
				if err != nil {
					logrus.Fatal(err)
				}
				err = enc.Encode(&node)
				if err != nil {
					logrus.Fatal(err)
				}
				fmt.Printf("%s\n", b.String())
			} else {
				nodes, err := c.NodeV1().List(ctx)
				if err != nil {
					logrus.Fatal(err)
				}
				fmt.Fprintf(wr, "%s\t%s\t%s\t%s\n", "NAME", "REVISION", "READY", "AGE")
				for _, n := range nodes {
					fmt.Fprintf(wr, "%s\t%d\t%t\t%s\n", n.GetMeta().GetName(), n.GetMeta().GetRevision(), n.GetStatus().GetReady(), time.Since(n.GetMeta().GetCreated().AsTime()).Round(1*time.Second))
				}
			}

			wr.Flush()
		},
	}

	return runCmd
}
