package get

import (
	"bytes"
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
			defer c.Close()

			if len(args) == 1 {
				var b bytes.Buffer
				enc := yaml.NewEncoder(&b)
				enc.SetIndent(2)
				cname := args[0]
				node, err := c.NodeV1().Get(context.Background(), cname)
				if err != nil {
					log.Fatal(err)
				}
				err = enc.Encode(&node)
				if err != nil {
					log.Fatal(err)
				}
				fmt.Printf("%s\n", b.String())
			} else {
				nodes, err := c.NodeV1().List(ctx)
				if err != nil {
					log.Fatal(err)
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
