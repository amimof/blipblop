// Package run provides ability to run resources
package run

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"

	"github.com/amimof/blipblop/api/services/containers/v1"
	"github.com/amimof/blipblop/api/types/v1"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/amimof/blipblop/pkg/cmdutil"
	"github.com/amimof/blipblop/pkg/networking"
)

var (
	image       string
	ports       []string
	wait        bool
	waitTimeout time.Duration
)

func NewCmdRun(cfg *client.Config) *cobra.Command {
	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Run a container",
		Long:  "Run a container. The run command required an image to be provided. The image must be in the format: registry/repo/image:tag",
		Example: `
# Run a prometheus container
bbctl run prometheus --image=docker.io/prom/prometheus:latest`,
		Args: cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				return err
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			cname := args[0]
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			tracer := otel.Tracer("bbctl")
			ctx, span := tracer.Start(ctx, "bbctl.run.container")
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

			// Setup ports
			var cports []*containers.PortMapping
			for _, p := range ports {

				pm, err := networking.ParsePorts(p)
				if err != nil {
					logrus.Fatal(err)
				}
				cports = append(cports, &containers.PortMapping{Name: pm.String(), HostPort: pm.Source, ContainerPort: pm.Destination})
			}

			err = c.ContainerV1().Create(ctx, &containers.Container{
				Meta: &types.Meta{
					Name: cname,
				},
				Config: &containers.Config{
					Image:        image,
					PortMappings: cports,
				},
			})
			if err != nil {
				logrus.Fatal(err)
			}

			if !viper.GetBool("wait") {
				fmt.Printf("Requested to run container %s\n", cname)
			}

			if viper.GetBool("wait") {

				dash := cmdutil.NewDashboard(args)
				go dash.Loop(ctx)

				// Fire off start operations concurrently
				go func(idx int, containerID string) {
					dash.Update(idx, func(s *cmdutil.ServiceState) {
						s.Text = "starting…"
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
				}(0, cname)

				dash.WaitAnd(cancel)

			}
		},
	}
	runCmd.Flags().StringVarP(
		&image,
		"image",
		"I",
		"",
		"Container image to run, must include the registry host",
	)
	runCmd.Flags().StringSliceVarP(
		&ports,
		"port",
		"p",
		[]string{},
		"Forward a local port to the container",
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
		"How long in seconds to wait for container to start before giving up",
	)
	if err := runCmd.MarkFlagRequired("image"); err != nil {
		logrus.Fatal(err)
	}
	return runCmd
}
