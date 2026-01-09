package delete

import (
	"context"
	"fmt"

	"github.com/amimof/voiyd/pkg/client"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewCmdDeleteNode(cfg *client.Config) *cobra.Command {
	runCmd := &cobra.Command{
		Use:     "node NAME [NAME...]",
		Short:   "Delete one or more nodes",
		Long:    "Delete one or more nodes",
		Example: `voiydctl delete node NAME`,
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
					logrus.Errorf("error closing client: %v", err)
				}
			}()

			for _, tname := range args {
				err = c.NodeV1().Delete(ctx, tname)
				if err != nil {
					logrus.Fatal(err)
				}
				if err != nil {
					logrus.Fatal(err)
				}
				fmt.Printf("Requested to delete node %s\n", tname)
			}
		},
	}

	return runCmd
}
