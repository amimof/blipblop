package start

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/amimof/voiyd/pkg/client"
	"github.com/amimof/voiyd/pkg/cmdutil"
	"github.com/amimof/voiyd/pkg/condition"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"

	typesv1 "github.com/amimof/voiyd/api/types/v1"
)

func NewCmdStartTask(cfg *client.Config) *cobra.Command {
	runCmd := &cobra.Command{
		Use:     "tasks NAME [NAME...]",
		Short:   "Start one or more tasks",
		Long:    "Start one or more tasks",
		Example: `voiydctl start task NAME`,
		Aliases: []string{"task"},
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
			ctx, span := tracer.Start(ctx, "voiydctl.start.task")
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

			// Start task one by one without waiting
			if !viper.GetBool("wait") {
				for _, tname := range args {
					_, err = c.TaskV1().Start(ctx, tname)
					if err != nil {
						logrus.Fatal(err)
					}
					fmt.Printf("Requested to start task %s\n", tname)
				}
			}

			// Start tasks in parallell and wait until they are running
			if viper.GetBool("wait") {

				dash := cmdutil.NewDashboard(args, cmdutil.WithWriter(cmdutil.DefaultTabWriter))
				go dash.Loop(ctx)

				for i, cname := range args {
					// Fire off start operations concurrently
					go func(idx int, taskID string) {
						_, err := c.TaskV1().Start(ctx, taskID)
						if err != nil {
							dash.FailMsg(idx, err.Error())
							return
						}

						dash.UpdateText(idx, "starting…")

						// Continously check task
						for {

							dash.FailAfterMsg(idx, viper.GetDuration("timeout"), "failed to start in time")

							task, err := c.TaskV1().Get(ctx, taskID)
							if err != nil {
								dash.FailMsg(idx, err.Error())
								return
							}

							image := task.GetConfig().GetImage()
							phase := task.GetStatus().GetPhase().GetValue()
							node := task.GetStatus().GetNode().GetValue()
							id := task.GetStatus().GetId().GetValue()

							dash.UpdateText(idx, fmt.Sprintf("%s…", phase))
							dash.UpdateDetails(idx, "Ready", fmt.Sprintf("%t", isReady(task.GetStatus().GetConditions())))
							dash.UpdateDetails(idx, "Image", image)
							dash.UpdateDetails(idx, "Node", node)
							dash.UpdateDetails(idx, "ID", id)

							if phase == "running" {
								dash.DoneMsg(idx, "started successfully")
								return
							}

							if strings.Contains(phase, "Err") {
								dash.FailMsg(idx, "failed to start")
								dash.UpdateDetails(idx, "Error", err.Error())
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

func isReady(t []*typesv1.Condition) bool {
	for _, cond := range t {
		if condition.Type(cond.GetType().GetValue()) == condition.TaskReady {
			if cond.GetStatus().GetValue() {
				return true
			}
		}
	}
	return false
}
