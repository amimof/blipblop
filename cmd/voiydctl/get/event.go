package get

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/amimof/voiyd/pkg/client"
	"github.com/amimof/voiyd/pkg/cmdutil"
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
		Example: `voiydctl get events`,
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

			tracer := otel.Tracer("voiydctl")
			ctx, span := tracer.Start(ctx, "voiydctl.get.event")
			defer span.End()

			// Setup client
			currentSrv, err := cfg.CurrentServer()
			if err != nil {
				logrus.Fatal(err)
			}
			c, err := client.New(currentSrv.Address, client.WithTLSConfigFromCfg(cfg))
			if err != nil {
				logrus.Fatalf("error setting up client: %v", err)
			}
			defer func() {
				if err := c.Close(); err != nil {
					logrus.Errorf("error closing client connection: %v", err)
				}
			}()

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
