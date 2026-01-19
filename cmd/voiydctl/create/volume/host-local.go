package volume

import (
	"context"

	"github.com/amimof/voiyd/api/services/volumes/v1"
	metav1 "github.com/amimof/voiyd/api/types/v1"
	"github.com/amimof/voiyd/pkg/client"
	"github.com/amimof/voiyd/pkg/cmdutil"
	"github.com/amimof/voiyd/services/volume"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewCmdCreateHostLocalVolume(cfg *client.Config) *cobra.Command {
	runCmd := &cobra.Command{
		Use:   "host-local NAME",
		Short: "Create a host-local volume",
		Long:  "Create a host-local volume",
		Example: `
# Create a host-local volume
voiydctl create volume host-local data01
`,
		Args: cobra.ExactArgs(1),
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
					logrus.Fatalf("error closing client connection: %v", err)
				}
			}()

			vname := args[0]

			err = c.VolumeV1().Create(
				ctx,
				&volumes.Volume{
					Version: volume.Version,
					Meta: &metav1.Meta{
						Name:   vname,
						Labels: cmdutil.ConvertKVStringsToMap(viper.GetStringSlice("resourceLabels")),
					},
					Config: &volumes.Config{
						HostLocal:    &volumes.HostLocal{},
						NodeSelector: cmdutil.ConvertKVStringsToMap(nodeSelector),
					},
				})
			if err != nil {
				logrus.Fatal(err)
			}

			logrus.Infof("request to create set %s successful", vname)
		},
	}

	return runCmd
}
