package start

import (
	"context"
	"fmt"

	"github.com/amimof/blipblop/pkg/client"
	"github.com/amimof/blipblop/pkg/cmdutil"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewCmdStartContainer(cfg *client.Config) *cobra.Command {
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
			c, err := client.New(ctx, cfg.CurrentServer().Address, client.WithTLSConfigFromCfg(cfg))
			if err != nil {
				logrus.Fatalf("error setting up client: %v", err)
			}
			defer c.Close()

			// Start containers
			for _, cname := range args {
				phase := ""
				_, err = c.ContainerV1().Start(ctx, cname)
				if err != nil {
					logrus.Fatal(err)
				}

				fmt.Printf("Requested to start container %s\n", cname)

				if viper.GetBool("wait") {
					fmt.Println("Waiting for container to start")
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
						if ctr.GetStatus().GetPhase() == "running" {
							stop()
						}
						return nil
					})
					if err != nil {
						logrus.Fatal(err)
					}
					fmt.Printf("Container %s started\n", cname)
				}
			}
		},
	}
	return runCmd
}
