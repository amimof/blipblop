package volume

import (
	"context"

	"github.com/amimof/blipblop/api/services/volumes/v1"
	metav1 "github.com/amimof/blipblop/api/types/v1"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewCmdCreateHostLocalVolume(cfg *client.Config) *cobra.Command {
	runCmd := &cobra.Command{
		Use:   "host-local NAME",
		Short: "Create a volume",
		Long:  "Create a volume",
		Example: `
# Create a host-local volume
bbctl create volume host-local data01
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
			c, err := client.New(cfg.CurrentServer().Address, client.WithTLSConfigFromCfg(cfg))
			if err != nil {
				logrus.Fatalf("error setting up client: %v", err)
			}
			defer func() {
				if err := c.Close(); err != nil {
					logrus.Fatalf("error closing client connection: %v", err)
				}
			}()

			vname := args[1]

			err = c.VolumeV1().Create(
				ctx,
				&volumes.Volume{
					Meta: &metav1.Meta{
						Name: vname,
					},
					Config: &volumes.Config{
						HostLocal: &volumes.HostLocal{},
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
