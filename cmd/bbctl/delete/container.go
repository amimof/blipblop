package delete

import (
	"context"
	"log"

	"github.com/amimof/blipblop/pkg/client"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/sirupsen/logrus"
)

func NewCmdDeleteContainer() *cobra.Command {
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
		Run: func(_ *cobra.Command, args []string) {
			server := viper.GetString("server")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Setup our client
			c, err := client.New(ctx, server)
			if err != nil {
				logrus.Fatal(err)
			}
			defer c.Close()

			cname := args[0]
			ctr, err := c.ContainerV1().Get(ctx, cname)
			if err != nil {
				log.Fatal(err)
			}
			err = c.ContainerV1().Delete(context.Background(), ctr.GetMeta().GetName())
			if err != nil {
				log.Fatal(err)
			}
			log.Printf("request to delete container %s successful", ctr.GetMeta().GetName())
		},
	}

	return runCmd
}
