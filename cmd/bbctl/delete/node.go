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
			ctx := context.Background()

			// Setup our client
			c, err := client.New(server)
			if err != nil {
				logrus.Fatal(err)
			}

			cname := args[0]
			ctr, err := c.NodeV1().GetNode(ctx, cname)
			if err != nil {
				log.Fatal(err)
			}
			err = c.NodeV1().DeleteNode(context.Background(), ctr.Name)
			if err != nil {
				log.Fatal(err)
			}
			log.Printf("request to delete node %s successful", ctr.GetName())
		},
	}

	return runCmd
}
