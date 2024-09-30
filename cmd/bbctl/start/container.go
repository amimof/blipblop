package start

import (
	"context"

	"github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewCmdStartContainer(c *client.ClientSet) *cobra.Command {
	runCmd := &cobra.Command{
		Use:     "container",
		Short:   "Start a container",
		Long:    "Start a container",
		Example: `bbctl start container NAME`,
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
			c, err := client.New(ctx, viper.GetString("server"), client.WithTLSConfigFromFlags(cmd.Flags()))
			if err != nil {
				logrus.Fatalf("error setting up client: %v", err)
			}

			cname := args[0]
			ctr, err := c.ContainerV1().Get(ctx, cname)
			if err != nil {
				logrus.Fatal(err)
			}

			logrus.Infof("requested to start container %s", ctr.GetMeta().GetName())

			_, err = c.ContainerV1().Start(context.Background(), ctr.GetMeta().GetName())
			if err != nil {
				logrus.Fatal(err)
			}

			if viper.GetBool("wait") {
				err = c.EventV1().Wait(events.EventType_ContainerStarted, cname)
				if err != nil {
					logrus.Fatal(err)
				}
				logrus.Infof("successfully started container %s", cname)
			}
		},
	}
	return runCmd
}
