package delete

import (
	"context"

	"github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewCmdDeleteContainer(cfg *client.Config) *cobra.Command {
	runCmd := &cobra.Command{
		Use:     "container",
		Short:   "Delete a container",
		Long:    "Delete a container",
		Example: `bbctl delete container NAME`,
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

			// Setup client
			c, err := client.New(ctx, cfg.CurrentServer().Address, client.WithTLSConfigFromCfg(cfg))
			if err != nil {
				logrus.Fatalf("error setting up client: %v", err)
			}
			defer c.Close()

			cname := args[0]
			ctr, err := c.ContainerV1().Get(ctx, cname)
			if err != nil {
				logrus.Fatal(err)
			}

			logrus.Infof("requested to delete container %s", ctr.GetMeta().GetName())

			go func() {
				err = c.ContainerV1().Delete(ctx, cname)
				if err != nil {
					logrus.Fatal(err)
				}
			}()

			if viper.GetBool("wait") {
				err = c.EventV1().Wait(events.EventType_ContainerDeleted, cname)
				if err != nil {
					logrus.Fatal(err)
				}
				logrus.Infof("successfully deleted container %s", cname)
			}
		},
	}

	return runCmd
}
