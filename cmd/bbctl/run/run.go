// Package run provides ability to run resources
package run

import (
	"context"
	"fmt"

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
	image              string
	ports              []string
	wait               bool
	waitTimeoutSeconds uint64
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

			phase := ""

			fmt.Printf("Requested to run container %s\n", cname)

			if viper.GetBool("wait") {
				fmt.Println("Waiting for container to start")
				spinner := cmdutil.NewSpinner(cmdutil.WithPrefix(&phase))
				spinner.Start()
				defer spinner.Stop()

				// Periodically get container phase
				err = cmdutil.Watch(ctx, cname, func(stop cmdutil.StopFunc) error {
					ctr, err := c.ContainerV1().Get(ctx, cname)
					if err != nil {
						logrus.Fatal(err)
					}
					phase = cmdutil.FormatPhase(ctr.GetStatus().GetPhase().String())
					if ctr.GetStatus().GetPhase().String() == "running" {
						stop()
					}
					return nil
				})
				if err != nil {
					logrus.Fatal(err)
				}
				fmt.Printf("Container %s started\n", cname)
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
		false,
		"Wait for command to finish",
	)
	runCmd.PersistentFlags().Uint64VarP(
		&waitTimeoutSeconds,
		"timeout",
		"",
		30,
		"How long in seconds to wait for container to start before giving up",
	)
	if err := runCmd.MarkFlagRequired("image"); err != nil {
		logrus.Fatal(err)
	}
	return runCmd
}
