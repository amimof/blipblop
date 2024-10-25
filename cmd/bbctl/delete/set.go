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

			fmt.Printf("Requested to delete containerset %s\n", cname)

			err = c.ContainerSetV1().Delete(ctx, cname)
			if err != nil {
				logrus.Fatal(err)
			}

			fmt.Printf("ContainerSet %s deleted\n", cname)
		},
	}

	return runCmd
}
