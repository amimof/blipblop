package create

import (
	"context"

	"github.com/amimof/blipblop/api/services/containers/v1"
	containersetsv1 "github.com/amimof/blipblop/api/services/containersets/v1"
	metav1 "github.com/amimof/blipblop/api/types/v1"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewCmdCreateSet(cfg *client.Config) *cobra.Command {
	runCmd := &cobra.Command{
		Use:     "set NAME",
		Short:   "Create a set",
		Long:    "Create a set",
		Example: `bbctl create set NAME`,
		Args:    cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				return err
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Get labels, for some reason we cannot use viper here. Viper returns an empty map
			// l, err := createCmd.PersistentFlags().GetStringToString("labels")
			// if err != nil {
			// 	log.Fatalf("error retrieving labels flag: %v", err)
			// }

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

			cname := args[0]

			err = c.ContainerSetV1().Create(
				ctx,
				&containersetsv1.ContainerSet{
					Meta: &metav1.Meta{
						Name: cname,
					},
					Template: &containers.Config{
						Image: "docker.io/library/nginx:latest",
					},
				})
			if err != nil {
				logrus.Fatal(err)
			}
			logrus.Infof("request to create set %s successful", cname)
		},
	}

	return runCmd
}
