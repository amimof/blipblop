package get

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/amimof/blipblop/api/services/volumes/v1"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/amimof/blipblop/pkg/cmdutil"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
	"google.golang.org/protobuf/encoding/protojson"
)

func NewCmdGetVolume(cfg *client.Config) *cobra.Command {
	runCmd := &cobra.Command{
		Use:     "volume",
		Short:   "Get a volume",
		Long:    "Get a volume",
		Example: `bbctl get volume`,
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
			ctx, span := tracer.Start(ctx, "bbctl.get.volume")
			defer span.End()

			// Setup client
			c, err := client.New(cfg.CurrentServer().Address, client.WithTLSConfigFromCfg(cfg))
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

			if len(args) == 0 {
				volumes, err := c.VolumeV1().List(ctx)
				if err != nil {
					logrus.Fatal(err)
				}
				for _, c := range volumes {

					numReady := fmt.Sprintf("%s/%d", getVolumeReadyStr(c.GetStatus().GetControllers()), len(c.GetStatus().GetControllers()))

					_, _ = fmt.Fprintf(wr, "%s\t%d\t%s\t%s\t%s\n",
						c.GetMeta().GetName(),
						c.GetMeta().GetRevision(),
						numReady,
						"host-local",
						cmdutil.FormatDuration(time.Since(c.GetMeta().GetCreated().AsTime())),
					)
				}
			}

			_ = wr.Flush()

			if len(args) == 1 {
				cname := args[0]
				container, err := c.VolumeV1().Get(context.Background(), cname)
				if err != nil {
					logrus.Fatal(err)
				}

				marshaler := protojson.MarshalOptions{
					EmitUnpopulated: false,
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

func getVolumeReadyStr(m map[string]*volumes.ControllerStatus) string {
	ready := 0
	for _, v := range m {
		if v.GetReady().GetValue() {
			ready++
		}
	}
	return fmt.Sprintf("%d", ready)
}
