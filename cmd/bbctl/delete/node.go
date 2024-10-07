package delete

import (
	"context"

	"github.com/amimof/blipblop/pkg/client"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewCmdDeleteNode(cfg *client.Config) *cobra.Command {
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
			ctr, err := c.NodeV1().Get(ctx, cname)
			if err != nil {
				logrus.Fatal(err)
			}
			err = c.NodeV1().Delete(context.Background(), ctr.GetMeta().GetName())
			if err != nil {
				logrus.Fatal(err)
			}
			logrus.Infof("request to delete node %s successful", ctr.GetMeta().GetName())
		},
	}

	return runCmd
}
