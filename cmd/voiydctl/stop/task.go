package stop

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

func NewCmdStopTask(cfg *client.Config) *cobra.Command {
	runCmd := &cobra.Command{
		Use:     "tasks NAME [NAME...]",
		Short:   "Stop one or more tasks",
		Long:    "Stop one or more tasks",
		Example: `voiydctl stop task NAME`,
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
						if err = c.TaskV1().Kill(ctx, tname); err != nil {
							logrus.Fatal(err)
						}
					} else {
						if err = c.TaskV1().Stop(ctx, tname); err != nil {
							logrus.Fatal(err)
						}
					}

					fmt.Printf("requested to stop task %s\n", tname)
				}
			}

			// Send stop or kill for each task in args and wait for them all to stop
			if viper.GetBool("wait") {

				dash, err := cmdutil.NewDashboard(args, cmdutil.WithHeader("Stopping task"))
				if err != nil {
					logrus.Fatal(err)
				}

				appCtx := cmd.Context()
				go dash.Loop(appCtx)

				for i, cname := range args {
					// Fire off start operations concurrently
					go func(idx int, taskID string) {
						dash.FailAfterMsg(idx, waitTimeout, "timeout reached")
						if viper.GetBool("force") {
							if err = c.TaskV1().Kill(ctx, cname); err != nil {
								fmt.Printf("Error stopping task: %v", err)
								return
							}
						} else {
							if err = c.TaskV1().Stop(ctx, cname); err != nil {
								fmt.Printf("Error stopping task: %v", err)
								return
							}
						}

						// Continously check task
						for {

							task, werr := c.TaskV1().Get(ctx, taskID)
							if werr != nil {
								fmt.Printf("Error stopping task: %v", err)
								return
							}

							dash.SetTask(idx, task)

							// Wait until retry
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
