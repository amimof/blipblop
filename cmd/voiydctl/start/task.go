package start

import (
	"context"
	"fmt"
	"time"

	"github.com/amimof/voiyd/pkg/client"
	"github.com/amimof/voiyd/pkg/cmdutil"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
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
					err = c.TaskV1().Start(ctx, tname)
					if err != nil {
						logrus.Fatal(err)
					}
					fmt.Printf("Requested to start task %s\n", tname)
				}
			}

			// Start tasks in parallell and wait until they are running
			if viper.GetBool("wait") {

				dash, err := cmdutil.NewDashboard(args, cmdutil.WithHeader("Starting task"))
				if err != nil {
					logrus.Fatal(err)
				}

				appCtx := cmd.Context()
				go dash.Loop(appCtx)

				for i, cname := range args {
					// Fire off start operations concurrently
					go func(idx int, taskID string) {
						dash.FailAfterMsg(idx, waitTimeout, "timeout reached")

						err := c.TaskV1().Start(ctx, taskID)
						if err != nil {
							dash.FailMsg(idx, err.Error())
							return
						}

						// Continously check task
						for {

							task, err := c.TaskV1().Get(ctx, taskID)
							if err != nil {
								dash.FailMsg(idx, err.Error())
								return
							}

							dash.SetTask(idx, task)

							time.Sleep(250 * time.Millisecond)
						}
					}(i, cname)
				}

				dash.Wait()

			}
		},
	}
	return runCmd
}
