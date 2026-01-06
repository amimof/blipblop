package stop

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/amimof/voiyd/pkg/client"
	"github.com/amimof/voiyd/pkg/cmdutil"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
)

func NewCmdStopTask(cfg *client.Config) *cobra.Command {
	runCmd := &cobra.Command{
		Use:     "task NAME",
		Short:   "Stop a task",
		Long:    "Stop a task",
		Example: `voiydctl stop task NAME`,
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

			tracer := otel.Tracer("voiydctl")
			ctx, span := tracer.Start(ctx, "voiydctl.stop.task")
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

			// Send stop or kill for each task in args without waiting
			if !viper.GetBool("wait") {
				for _, tname := range args {

					if viper.GetBool("force") {
						if _, err = c.TaskV1().Kill(ctx, tname); err != nil {
							logrus.Fatal(err)
						}
					} else {
						if _, err = c.TaskV1().Stop(ctx, tname); err != nil {
							logrus.Fatal(err)
						}
					}

					fmt.Printf("requested to stop task %s\n", tname)
				}
			}

			// Send stop or kill for each task in args and wait for them all to stop
			if viper.GetBool("wait") {
				dash := cmdutil.NewDashboard(args)
				go dash.Loop(ctx)

				for i, cname := range args {
					// Fire off start operations concurrently
					go func(idx int, taskID string) {
						if viper.GetBool("force") {
							if _, err = c.TaskV1().Kill(ctx, cname); err != nil {
								dash.FailMsg(idx, fmt.Sprintf("failed to start: %v", err))
								return
							}
						} else {
							if _, err = c.TaskV1().Stop(ctx, cname); err != nil {
								dash.FailMsg(idx, fmt.Sprintf("failed to start: %v", err))
								return
							}
						}

						dash.UpdateText(idx, "stopping…")

						// Continously check task
						for {

							dash.FailAfterMsg(idx, viper.GetDuration("timeout"), "failed to start in time")

							task, werr := c.TaskV1().Get(ctx, taskID)
							if werr != nil {
								dash.FailMsg(idx, werr.Error())
								return
							}

							phase := task.GetStatus().GetPhase().GetValue()
							status := task.GetStatus().GetStatus().GetValue()

							dash.UpdateText(idx, fmt.Sprintf("%s…", phase))

							if status != "" {
								dash.UpdateDetails(idx, "Status", status)
							}

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
