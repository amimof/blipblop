package stop

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

func NewCmdStopContainer(cfg *client.Config) *cobra.Command {
	runCmd := &cobra.Command{
		Use:     "container",
		Short:   "Stop a container",
		Long:    "Stop a container",
		Example: `bbctl stop container NAME`,
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
			ctx, span := tracer.Start(ctx, "bbctl.stop.container")
			defer span.End()

			// Setup client
			c, err := client.New(cfg.CurrentServer().Address, client.WithTLSConfigFromCfg(cfg))
			if err != nil {
				logrus.Fatalf("error setting up client: %v", err)
			}
			defer func() {
				if err := c.Close(); err != nil {
					logrus.Errorf("error closing client: %v", err)
				}
			}()

			// Send stop or kill for each container in args without waiting
			if !viper.GetBool("wait") {
				for _, cname := range args {

					if viper.GetBool("force") {
						if _, err = c.ContainerV1().Kill(ctx, cname); err != nil {
							logrus.Fatal(err)
						}
					} else {
						if _, err = c.ContainerV1().Stop(ctx, cname); err != nil {
							logrus.Fatal(err)
						}
					}

					fmt.Printf("requested to stop container %s\n", cname)
				}
			}

			// Send stop or kill for each container in args and wait for them all to stop
			if viper.GetBool("wait") {
				dash := cmdutil.NewDashboard(args)
				go dash.Loop(ctx)

				for i, cname := range args {
					// Fire off start operations concurrently
					go func(idx int, containerID string) {
						if viper.GetBool("force") {
							if _, err = c.ContainerV1().Kill(ctx, cname); err != nil {
								dash.FailMsg(idx, fmt.Sprintf("failed to start: %v", err))
								return
							}
						} else {
							if _, err = c.ContainerV1().Stop(ctx, cname); err != nil {
								dash.FailMsg(idx, fmt.Sprintf("failed to start: %v", err))
								return
							}
						}

						dash.Update(idx, func(s *cmdutil.ServiceState) {
							s.Text = "stopping…"
						})

						// Continously check container
						for {

							dash.FailAfterMsg(idx, viper.GetDuration("timeout"), "failed to start in time")

							ctr, werr := c.ContainerV1().Get(ctx, containerID)
							if werr != nil {
								dash.FailMsg(idx, werr.Error())
								return
							}

							phase := ctr.GetStatus().GetPhase().GetValue()
							dash.Update(idx, func(s *cmdutil.ServiceState) {
								s.Text = fmt.Sprintf("%s…", phase)
							})

							if phase == "stopped" {
								dash.DoneMsg(idx, "stopped successfully")
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
