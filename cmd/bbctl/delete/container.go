package delete

import (
	"context"
	"fmt"

	"github.com/amimof/blipblop/pkg/client"
	"github.com/amimof/blipblop/pkg/cmdutil"
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
			phase := ""

			fmt.Printf("Requested to delete container %s", cname)

			err = c.ContainerV1().Delete(ctx, cname)
			if err != nil {
				logrus.Fatal(err)
			}

			if viper.GetBool("wait") {
				fmt.Println("Waiting for container to stop")
				spinner := cmdutil.NewSpinner(cmdutil.WithPrefix(&phase))
				spinner.Start()
				defer spinner.Stop()

				// Periodically get container phase
				err = cmdutil.Watch(ctx, cname, func(stop cmdutil.StopFunc) error {
					ctr, err := c.ContainerV1().Get(ctx, cname)
					if err != nil {
						logrus.Fatal(err)
					}

					phase = cmdutil.FormatPhase(ctr.GetStatus().GetPhase())
					if ctr.GetStatus().GetPhase() == "Deleted" {
						stop()
					}
					return nil
				})
				if err != nil {
					logrus.Fatal(err)
				}
				fmt.Printf("Container %s deleted\n", cname)
			}
		},
	}

	return runCmd
}
