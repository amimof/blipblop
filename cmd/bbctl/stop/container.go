package stop

import (
	"context"
	"fmt"

	"github.com/amimof/blipblop/pkg/client"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewCmdStopContainer(cfg *client.Config) *cobra.Command {
	runCmd := &cobra.Command{
		Use:     "container",
		Short:   "Stop a container",
		Long:    "Stop a container",
		Example: `bbctl stop container NAME`,
		Args:    cobra.MinimumNArgs(1),
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
			c, err := client.New(ctx, cfg.CurrentServer().Address, client.WithTLSConfigFromCfg(cfg))
			if err != nil {
				logrus.Fatalf("error setting up client: %v", err)
			}
			defer c.Close()

			// Send stop request to server
			for _, cname := range args {

				if viper.GetBool("force") {
					if _, err = c.ContainerV1().Kill(ctx, cname); err != nil {
						logrus.Fatal(err)
					}
				}
				if _, err = c.ContainerV1().Stop(ctx, cname); err != nil {
					logrus.Fatal(err)
				}

				fmt.Printf("requested to stop container %s\n", cname)
			}
		},
	}

	return runCmd
}
