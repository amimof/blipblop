package delete

import (
	"context"
	"fmt"

	"github.com/amimof/voiyd/pkg/client"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
)

var purge bool

func NewCmdDeleteVolume(cfg *client.Config) *cobra.Command {
	runCmd := &cobra.Command{
		Use:   "volume NAME",
		Short: "Delete a volume",
		Long:  "Delete a volume",
		Example: `
# Delete the volume data01 and purge all content  
voiydctl delete volume data01 --purge`,
		Args: cobra.MinimumNArgs(1),
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
			ctx, span := tracer.Start(ctx, "voiydctl.delete.volume")
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
					logrus.Errorf("error closing client: %v", err)
				}
			}()

			for _, vname := range args {
				fmt.Printf("Requested to delete volume %s\n", vname)
				err = c.VolumeV1().Delete(ctx, vname)
				if err != nil {
					logrus.Fatal(err)
				}
			}
		},
	}

	runCmd.Flags().BoolVarP(
		&purge,
		"purge",
		"P",
		false,
		"Purge all contents of the destination directory on removal",
	)
	return runCmd
}
