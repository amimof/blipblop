package delete

import (
	"context"
	"log"

	"github.com/amimof/blipblop/pkg/client"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/sirupsen/logrus"
)

func NewCmdDeleteNode() *cobra.Command {
	runCmd := &cobra.Command{
		Use:     "node",
		Short:   "Delete a node",
		Long:    "Delete a node",
		Example: `bbctl delete node NAME`,
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
			ctr, err := c.NodeV1().Get(ctx, cname)
			if err != nil {
				log.Fatal(err)
			}
			err = c.NodeV1().Delete(context.Background(), ctr.GetMeta().GetName())
			if err != nil {
				log.Fatal(err)
			}
			log.Printf("request to delete node %s successful", ctr.GetMeta().GetName())
		},
	}

	return runCmd
}
