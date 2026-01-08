package delete

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

func NewCmdDeleteTask(cfg *client.Config) *cobra.Command {
	runCmd := &cobra.Command{
		Use:     "task",
		Short:   "Delete a task",
		Long:    "Delete a task",
		Example: `voiydctl delete task NAME`,
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
			ctx, span := tracer.Start(ctx, "voiydctl.delete.task")
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

			// Delete task one by one without waiting
			if !viper.GetBool("wait") {
				for _, tname := range args {
					err = c.TaskV1().Delete(ctx, tname)
					if err != nil {
						logrus.Fatal(err)
					}
					fmt.Printf("Requested to delete task %s\n", tname)
				}
			}

			// Stop tasks in parallell and wait until they are stopped before deleting them
			if viper.GetBool("wait") {
				dash := cmdutil.NewDashboard(args, cmdutil.WithWriter(cmdutil.DefaultTabWriter))
				go dash.Loop(ctx)
				for i, cname := range args {
					// Fire off delete operations concurrently
					go func(idx int, taskID string) {
						if viper.GetBool("force") {
							if _, err = c.TaskV1().Kill(ctx, cname); err != nil {
								dash.FailMsg(idx, fmt.Sprintf("failed to kill: %v", err))
								return
							}
						} else {
							if _, err = c.TaskV1().Stop(ctx, cname); err != nil {
								dash.FailMsg(idx, fmt.Sprintf("failed to stop: %v", err))
								return
							}
						}

						dash.UpdateText(idx, "stopping…")

						// Continously check task
						for {

							dash.FailAfterMsg(idx, viper.GetDuration("timeout"), "failed to stop in time")

							task, err := c.TaskV1().Get(ctx, taskID)
							if err != nil {
								dash.FailMsg(idx, err.Error())
								return
							}

							phase := task.GetStatus().GetPhase().GetValue()
							status := task.GetStatus().GetStatus().GetValue()

							dash.UpdateText(idx, fmt.Sprintf("%s…", phase))
							dash.UpdateDetails(idx, "Status", status)

							if phase == "stopped" {
								err = c.TaskV1().Delete(ctx, taskID)
								if err != nil {
									dash.FailMsg(idx, "failed to delete")
									dash.UpdateDetails(idx, "Error", err.Error())
									return
								}
								dash.DoneMsg(idx, "deleted successfully")
								return
							}

							if strings.Contains(phase, "Err") {
								dash.FailMsg(idx, "failed to delete")
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
