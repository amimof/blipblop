package get

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/amimof/blipblop/pkg/client"
	"github.com/amimof/blipblop/pkg/cmdutil"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

func NewCmdGetContainer(cfg *client.Config) *cobra.Command {
	runCmd := &cobra.Command{
		Use:     "container",
		Short:   "Get a container",
		Long:    "Get a container",
		Example: `bbctl get container`,
		Args:    cobra.MaximumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				return err
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
			defer cancel()

			// Setup client
			c, err := client.New(ctx, cfg.CurrentServer().Address, client.WithTLSConfigFromCfg(cfg))
			if err != nil {
				logrus.Fatalf("error setting up client: %v", err)
			}
			defer c.Close()

			// Setup writer
			wr := tabwriter.NewWriter(os.Stdout, 8, 8, 8, '\t', tabwriter.AlignRight)

			if len(args) == 0 {
				containers, err := c.ContainerV1().List(ctx)
				if err != nil {
					logrus.Fatal(err)
				}

				fmt.Fprintf(wr, "%s\t%s\t%s\t%s\t%s\t%s\n", "NAME", "REVISION", "PHASE", "STATUS", "NODE", "AGE")
				for _, c := range containers {
					var status string
					if c.GetStatus().GetExitStatus() != 0 {
						exitDuration := cmdutil.FormatDuration(time.Since(c.GetStatus().GetExitTime().AsTime()))
						status = fmt.Sprintf("Exited (%d) %s ago", c.GetStatus().GetExitStatus(), exitDuration)
					}
					fmt.Fprintf(wr, "%s\t%d\t%s\t%s\t%s\t%s\n",
						c.GetMeta().GetName(),
						c.GetMeta().GetRevision(),
						c.GetStatus().GetPhase(),
						status,
						c.GetStatus().GetNode(),
						cmdutil.FormatDuration(time.Since(c.GetMeta().GetCreated().AsTime())),
					)
				}
			}

			wr.Flush()

			if len(args) == 1 {
				var b bytes.Buffer
				enc := yaml.NewEncoder(&b)
				enc.SetIndent(2)
				cname := args[0]
				container, err := c.ContainerV1().Get(context.Background(), cname)
				if err != nil {
					logrus.Fatal(err)
				}
				err = enc.Encode(&container)
				if err != nil {
					logrus.Fatal(err)
				}
				fmt.Printf("%s\n", b.String())
			}
		},
	}

	return runCmd
}
