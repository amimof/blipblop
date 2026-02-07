package start

import (
	"context"
	"fmt"
	"os"
	"strconv"
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

				containers := cmdutil.NewContainers(len(args),
					map[string]any{},
					cmdutil.Layout{
						Dimensions: [2]int{76, 2},
						Padding:    [4]int{1, 2, 1, 2},
					},
					cmdutil.Style{
						Bg: cmdutil.ColorBgHiBlack,
					},
					cmdutil.NewElement(` {{ spinner | FgYellow }} Starting task {{ .Container.Name | FgBlue }}`),
					cmdutil.NewElement(`   Phase: {{ if eq .Container.Phase "Running" }}{{ .Container.Phase | FgGreen }}{{else}}{{ .Container.Phase | FgYellow }}{{end}}`),
					cmdutil.NewElement(`   Node: {{ .Container.Node | FgBlue }}`),
					cmdutil.NewElement(`   Pid: {{ .Container.Pid | FgBlue }}`),
					cmdutil.NewElement(`   ID: {{ .Container.ID | FgBlue }}`),
					cmdutil.NewElement(`   Image: {{ .Container.Image | FgBlue }}`),
				)

				app := cmdutil.NewApp(
					os.Stdout,
					map[string]any{},
					containers...,
				)

				appCtx, cancel := context.WithTimeout(cmd.Context(), time.Second*10)
				defer cancel()
				go app.Loop(appCtx)

				for i, cname := range args {
					// Fire off start operations concurrently
					go func(idx int, taskID string) {
						err := c.TaskV1().Start(ctx, taskID)
						if err != nil {
							fmt.Printf("Error starting task: %v", err)
							return
						}

						// Continously check task
						for {

							task, err := c.TaskV1().Get(ctx, taskID)
							if err != nil {
								fmt.Printf("Error starting task: %v", err)
								return
							}

							image := task.GetConfig().GetImage()
							phase := task.GetStatus().GetPhase().GetValue()
							node := task.GetStatus().GetNode().GetValue()
							id := task.GetStatus().GetId().GetValue()
							pid := strconv.Itoa(int(task.GetStatus().GetPid().GetValue()))

							md := map[string]any{
								"Name":  task.GetMeta().GetName(),
								"Phase": phase,
								"Pid":   pid,
								"Node":  node,
								"ID":    id,
								"Image": image,
							}

							containers[i].SetMetadata(md)

							time.Sleep(250 * time.Millisecond)
						}
					}(i, cname)
				}

				app.Wait()

			}
		},
	}
	return runCmd
}
