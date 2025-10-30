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
)

func NewCmdGetEvent(cfg *client.Config) *cobra.Command {
	runCmd := &cobra.Command{
		Use:     "event",
		Short:   "Get a events",
		Long:    "Get a events",
		Example: `bbctl get events`,
		Args:    cobra.MaximumNArgs(2),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				return err
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			tracer := otel.Tracer("bbctl")
			ctx, span := tracer.Start(ctx, "bbctl.get.event")
			defer span.End()

			// Setup client
			c, err := client.New(cfg.CurrentServer().Address, client.WithTLSConfigFromCfg(cfg))
			if err != nil {
				logrus.Fatalf("error setting up client: %v", err)
			}
			defer c.Close()

			// Setup writer
			wr := tabwriter.NewWriter(os.Stdout, 8, 8, 8, '\t', tabwriter.AlignRight)

			// List all events
			_, _ = fmt.Fprintf(wr, "%s\t%s\n", "TYPE", "AGE")
			if len(args) == 1 {
				events, err := c.EventV1().List(ctx)
				if err != nil {
					logrus.Fatal(err)
				}

				for _, event := range events {
					_, _ = fmt.Fprintf(wr, "%s\t%s\n",
						event.GetType().String(),
						cmdutil.FormatDuration(time.Since(event.GetMeta().GetCreated().AsTime())),
					)
				}

			}

			_ = wr.Flush()
		},
	}

	return runCmd
}
