package start

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/amimof/blipblop/pkg/client"
	"github.com/amimof/blipblop/pkg/cmdutil"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
)

func NewCmdStartContainer(cfg *client.Config) *cobra.Command {
	runCmd := &cobra.Command{
		Use:     "container",
		Short:   "Start a container",
		Long:    "Start a container",
		Example: `bbctl start container NAME`,
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

			tracer := otel.Tracer("bbctl")
			ctx, span := tracer.Start(ctx, "bbctl.start.container")
			defer span.End()

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

			// Start container one by one without waiting
			if !viper.GetBool("wait") {
				for _, cname := range args {
					_, err = c.ContainerV1().Start(ctx, cname)
					if err != nil {
						logrus.Fatal(err)
					}
					fmt.Printf("Requested to start container %s\n", cname)
				}
			}

			// Start containers in parallell and wait until they are running
			if viper.GetBool("wait") {

				dash := cmdutil.NewDashboard(args)
				go dash.Loop(ctx)

				for i, cname := range args {
					// Fire off start operations concurrently
					go func(idx int, containerID string) {
						_, err := c.ContainerV1().Start(ctx, containerID)
						if err != nil {
							dash.FailMsg(idx, err.Error())
							return
						}

						dash.Update(idx, func(s *cmdutil.ServiceState) {
							s.Text = "starting…"
						})

						// Continously check container
						for {

							dash.FailAfterMsg(idx, viper.GetDuration("timeout"), "failed to start in time")

							ctr, err := c.ContainerV1().Get(ctx, containerID)
							if err != nil {
								dash.FailMsg(idx, err.Error())
								return
							}

							phase := ctr.GetStatus().GetPhase().GetValue()
							dash.Update(idx, func(s *cmdutil.ServiceState) {
								s.Text = fmt.Sprintf("%s…", phase)
							})

							if phase == "running" {
								dash.DoneMsg(idx, "started successfully")
								return
							}

							if strings.Contains(phase, "Err") {
								dash.FailMsg(idx, "failed to start")
								return
							}

							// Wait until retry
							time.Sleep(250 * time.Millisecond)
						}
					}(i, cname)
				}

				dash.WaitAnd(cancel)

			}
		},
	}
	return runCmd
}
