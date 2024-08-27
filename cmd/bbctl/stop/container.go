package stop

import (
	"context"
	"log"

	"github.com/amimof/blipblop/pkg/client"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewCmdStopContainer() *cobra.Command {
	runCmd := &cobra.Command{
		Use:     "container",
		Short:   "Stop a container",
		Long:    "Stop a container",
		Example: `bbctl stop container NAME`,
		Args:    cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				return err
			}
			return nil
		},
		Run: func(_ *cobra.Command, args []string) {
			server := viper.GetString("server")
			ctx := context.Background()

			// Setup our client
			c, err := client.New(ctx, server)
			if err != nil {
				logrus.Fatal(err)
			}

			cname := args[0]
			ctr, err := c.ContainerV1().GetContainer(ctx, cname)
			if err != nil {
				log.Fatal(err)
			}
			err = c.ContainerV1().KillContainer(context.Background(), ctr.Name)
			if err != nil {
				log.Fatal(err)
			}
			log.Printf("request to stop container %s successful", cname)
		},
	}

	return runCmd
}
