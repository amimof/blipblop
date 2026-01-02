// Package delete provides ability to delete resources from the server
package delete

import (
	"context"
	"fmt"

	"github.com/amimof/blipblop/pkg/client"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewCmdDeleteContainerSet(cfg *client.Config) *cobra.Command {
	runCmd := &cobra.Command{
		Use:     "containerset",
		Short:   "Delete a containerset",
		Long:    "Delete a containerset",
		Example: `bbctl delete containerset NAME`,
		Args:    cobra.MinimumNArgs(1),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
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

			for _, cname := range args {

				fmt.Printf("Requested to delete containerset %s\n", cname)

				err = c.ContainerSetV1().Delete(ctx, cname)
				if err != nil {
					logrus.Fatal(err)
				}

				fmt.Printf("ContainerSet %s deleted\n", cname)
			}
		},
	}

	return runCmd
}
