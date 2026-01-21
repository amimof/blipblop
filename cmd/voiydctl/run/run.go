// Package run provides ability to run resources
package run

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/amimof/voiyd/api/types/v1"
	"github.com/amimof/voiyd/pkg/client"
	"github.com/amimof/voiyd/pkg/cmdutil"
	"github.com/amimof/voiyd/pkg/condition"
	"github.com/amimof/voiyd/pkg/networking"

	tasksv1 "github.com/amimof/voiyd/api/services/tasks/v1"
)

var (
	image        string
	ports        []string
	user         string
	privileged   bool
	wait         bool
	waitTimeout  time.Duration
	capAdd       []string
	capDrop      []string
	env          []string
	labels       []string
	nodeSelector []string
)

func NewCmdRun() *cobra.Command {
	var cfg client.Config
	runCmd := &cobra.Command{
		Use:   "run NAME",
		Short: "Run a task",
		Long:  "Run a task. The run command required an image to be provided. The image must be in the format: registry/repo/image:tag",
		Example: `
# Run a prometheus task
voiydctl run prometheus --image=docker.io/prom/prometheus:latest

# Run a task exposing port to the host
voiydctl run nginx --image=docker.io/library/nginx:latest -p 8080:80

# Run a task as user and group
voiydctl run nginx --image=docker.io/library/nginx:latest -p 8080:80 --user 1024:1024
`,

		Args: cobra.ExactArgs(1),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				return err
			}
			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				return err
			}
			if err := viper.ReadInConfig(); err != nil {
				logrus.Fatalf("error reading config: %v", err)
			}
			if err := viper.Unmarshal(&cfg); err != nil {
				logrus.Fatalf("error decoding config into struct: %v", err)
			}
			if err := cfg.Validate(); err != nil {
				logrus.Fatal(err)
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			tname := args[0]
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			tracer := otel.Tracer("voiydctl")
			ctx, span := tracer.Start(ctx, "voiydctl.run.task")
			defer span.End()

			// Setup client
			currentSrv, err := cfg.CurrentServer()
			if err != nil {
				logrus.Fatal(err)
			}
			c, err := client.New(currentSrv.Address, client.WithTLSConfigFromCfg(&cfg))
			if err != nil {
				logrus.Fatalf("error setting up client: %v", err)
			}
			defer func() {
				if err := c.Close(); err != nil {
					logrus.Errorf("error closing client: %v", err)
				}
			}()

			// Setup ports
			var tports []*tasksv1.PortMapping
			for _, p := range ports {

				pm, err := networking.ParsePorts(p)
				if err != nil {
					logrus.Fatal(err)
				}
				tports = append(tports, &tasksv1.PortMapping{Name: pm.String(), HostPort: pm.Source, TargetPort: pm.Destination})
			}

			err = c.TaskV1().Create(ctx, &tasksv1.Task{
				Meta: &types.Meta{
					Name:   tname,
					Labels: cmdutil.ReadKVStringsMapFromLabel(labels),
				},
				Config: &tasksv1.Config{
					Image:        image,
					PortMappings: tports,
					User:         user,
					Privileged:   privileged,
					Capabilities: &tasksv1.Capabilities{
						Add:  capAdd,
						Drop: capDrop,
					},
					NodeSelector: cmdutil.ReadKVStringsMapFromLabel(nodeSelector),
				},
			})
			if err != nil {
				logrus.Fatal(err)
			}

			if !viper.GetBool("wait") {
				fmt.Printf("Requested to run task %s\n", tname)
			}

			if viper.GetBool("wait") {

				dash := cmdutil.NewDashboard(args, cmdutil.WithWriter(cmdutil.DefaultTabWriter))
				go dash.Loop(ctx)

				// Fire off start operations concurrently
				go func(idx int, taskID string) {
					dash.UpdateText(idx, "starting…")

					// Continously check task
					for {

						dash.FailAfterMsg(idx, viper.GetDuration("timeout"), "failed to start in time")

						task, werr := c.TaskV1().Get(ctx, taskID)
						if werr != nil {
							dash.FailMsg(idx, werr.Error())
							return
						}

						// image := task.GetConfig().GetImage()
						// phase := task.GetStatus().GetPhase().GetValue()
						// node := task.GetStatus().GetNode().GetValue()
						// id := task.GetStatus().GetId().GetValue()
						// reason := task.GetStatus().GetReason().GetValue()

						dash.UpdateText(idx, fmt.Sprintf("%s…", "starting"))

						for _, cond := range task.GetStatus().GetConditions() {
							dash.UpdateDetails(idx, cond.GetType().GetValue(), fmt.Sprintf("%t", cond.GetStatus().GetValue()))
							// dash.UpdateDetails(idx, "Node", node)
							// dash.UpdateDetails(idx, "ID", id)
							// dash.UpdateDetails(idx, "Reason", reason)
							if condition.Type(cond.GetType().GetValue()) == condition.TaskReady && cond.GetStatus() == wrapperspb.Bool(true) {
								dash.DoneMsg(idx, "started successfully")
								return
							}
						}

						// if strings.Contains(phase, "Err") {
						// 	dash.FailMsg(idx, "failed to start")
						// 	dash.UpdateDetails(idx, "Error", err.Error())
						// 	return
						// }

						// Wait until retry
						time.Sleep(250 * time.Millisecond)
					}
				}(0, tname)

				dash.WaitAnd(cancel)

			}
		},
	}
	runCmd.Flags().StringVarP(
		&image,
		"image",
		"i",
		"",
		"Container image to run, must include the registry host",
	)
	runCmd.Flags().StringSliceVarP(
		&ports,
		"port",
		"p",
		[]string{},
		"Forward a local port to the task",
	)
	runCmd.Flags().StringVarP(
		&user,
		"user",
		"u",
		"",
		"Username or UID (format: <name|uid>[:<group|gid>])",
	)
	runCmd.Flags().BoolVar(
		&privileged,
		"privileged",
		false,
		"Give extended privileges to the task",
	)
	runCmd.Flags().StringSliceVar(
		&capAdd,
		"cap-add",
		[]string{},
		"Add Linux capabilities",
	)
	runCmd.Flags().StringSliceVar(
		&capDrop,
		"cap-drop",
		[]string{},
		"Drop Linux capabilities",
	)
	runCmd.Flags().StringArrayVarP(
		&env,
		"env",
		"e",
		[]string{},
		"Set environment variables",
	)
	runCmd.Flags().StringArrayVarP(
		&labels,
		"label",
		"l",
		[]string{},
		"Set task metadata labels",
	)
	runCmd.Flags().StringArrayVar(
		&nodeSelector,
		"node-selector",
		[]string{},
		"Set task node selector",
	)
	runCmd.PersistentFlags().BoolVarP(
		&wait,
		"wait",
		"w",
		true,
		"Wait for command to finish",
	)
	runCmd.PersistentFlags().DurationVarP(
		&waitTimeout,
		"timeout",
		"",
		time.Second*30,
		"How long in seconds to wait for task to start before giving up",
	)
	if err := runCmd.MarkFlagRequired("image"); err != nil {
		logrus.Fatal(err)
	}
	return runCmd
}
