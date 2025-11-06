package get

import (
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
	"google.golang.org/protobuf/encoding/protojson"
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

			tracer := otel.Tracer("bbctl")
			ctx, span := tracer.Start(ctx, "bbctl.get.container")
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
				containers, err := c.ContainerV1().List(ctx)
				if err != nil {
					logrus.Fatal(err)
				}

				_, _ = fmt.Fprintf(wr, "%s\t%s\t%s\t%s\t%s\t%s\n", "NAME", "REVISION", "PHASE", "STATUS", "NODE", "AGE")
				for _, c := range containers {
					var status string
					if c.GetStatus().GetTask().GetExitCode().GetValue() != 0 {
						exitDuration := cmdutil.FormatDuration(time.Since(c.GetStatus().GetTask().GetExitTime().AsTime()))
						status = fmt.Sprintf("Exited (%d) %s ago", c.GetStatus().GetTask().GetExitCode().GetValue(), exitDuration)
					}
					_, _ = fmt.Fprintf(wr, "%s\t%d\t%s\t%s\t%s\t%s\n",
						c.GetMeta().GetName(),
						c.GetMeta().GetRevision(),
						c.GetStatus().GetPhase().GetValue(),
						status,
						c.GetStatus().GetNode().GetValue(),
						cmdutil.FormatDuration(time.Since(c.GetMeta().GetCreated().AsTime())),
					)
				}
			}

			_ = wr.Flush()

			if len(args) == 1 {
				cname := args[0]
				container, err := c.ContainerV1().Get(context.Background(), cname)
				if err != nil {
					logrus.Fatal(err)
				}

				marshaler := protojson.MarshalOptions{
					EmitUnpopulated: true,
					Indent:          "  ",
				}
				b, err := marshaler.Marshal(container)
				if err != nil {
					logrus.Fatal(err)
				}
				fmt.Printf("%s\n", string(b))
			}
		},
	}

	return runCmd
}
