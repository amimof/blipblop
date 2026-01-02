package volume

import (
	"context"
	"io"
	"os"

	"github.com/amimof/blipblop/api/services/volumes/v1"
	metav1 "github.com/amimof/blipblop/api/types/v1"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/amimof/blipblop/services/volume"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var fileName string

func NewCmdCreateTemplateVolume(cfg *client.Config) *cobra.Command {
	runCmd := &cobra.Command{
		Use:   "template NAME FILE",
		Short: "Create a template volume",
		Long:  "Creates a template volume with provided file as its content",
		Example: `
# Create a template volume from file
bbctl create volume template app-config config.yaml

# Create a template volume with specific file name
bbctl create volume template app-config config.yaml --file-name prometheus.yaml
`,
		Args: cobra.ExactArgs(2),
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
			fname := args[1]

			// Read content of the file

			f, err := os.OpenFile(fname, os.O_RDONLY, 0o644)
			if err != nil {
				logrus.Fatal(err)
			}

			b, err := io.ReadAll(f)
			if err != nil {
				logrus.Fatal(err)
			}

			err = c.VolumeV1().Create(
				ctx,
				&volumes.Volume{
					Version: volume.Version,
					Meta: &metav1.Meta{
						Name: vname,
					},
					Config: &volumes.Config{
						Template: &volumes.Template{
							Name: fname,
							Data: string(b),
						},
					},
				})
			if err != nil {
				logrus.Fatal(err)
			}

			logrus.Infof("request to create set %s successful", vname)
		},
	}

	runCmd.PersistentFlags().StringVarP(
		&fileName,
		"file-name",
		"f",
		"",
		"File name. Defaults to the name of the file read from fs",
	)

	return runCmd
}
