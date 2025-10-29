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
	"go.opentelemetry.io/otel"
	"gopkg.in/yaml.v3"
)

func NewCmdGetContainerSet(cfg *client.Config) *cobra.Command {
	runCmd := &cobra.Command{
		Use:     "containerset",
		Short:   "Get a containerset",
		Long:    "Get a containerset",
		Example: `bbctl get containerset`,
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

			tracer := otel.Tracer("bbctl")
			ctx, span := tracer.Start(ctx, "bbctl.get.containerset")
			defer span.End()

			// Setup client
			c, err := client.New(cfg.CurrentServer().Address, client.WithTLSConfigFromCfg(cfg))
			if err != nil {
				logrus.Fatalf("error setting up client: %v", err)
			}
			defer c.Close()

			// Setup writer
			wr := tabwriter.NewWriter(os.Stdout, 8, 8, 8, '\t', tabwriter.AlignRight)

			if len(args) == 0 {
				containers, err := c.ContainerSetV1().List(ctx)
				if err != nil {
					logrus.Fatal(err)
				}
				fmt.Fprintf(wr, "%s\t%s\t%s\n", "NAME", "REVISION", "AGE")
				for _, c := range containers {
					fmt.Fprintf(wr, "%s\t%d\t%s\n",
						c.GetMeta().GetName(),
						c.GetMeta().GetRevision(),
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
				container, err := c.ContainerSetV1().Get(context.Background(), cname)
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
