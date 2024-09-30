package run

import (
	"context"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/amimof/blipblop/api/services/containers/v1"
	"github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/api/types/v1"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/amimof/blipblop/pkg/networking"
)

var (
	image              string
	ports              []string
	wait               bool
	waitTimeoutSeconds uint64
)

func NewCmdRun() *cobra.Command {
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

			// Setup client
			c, err := client.New(ctx, viper.GetString("server"), client.WithTLSConfigFromFlags(cmd.Flags()))
			if err != nil {
				logrus.Fatalf("error setting up client: %v", err)
			}
			defer c.Close()

			// Setup ports
			var cports []*containers.PortMapping
			for _, p := range ports {

				pm, err := networking.ParsePorts(p)
				if err != nil {
					logrus.Fatal(err)
				}
				cports = append(cports, &containers.PortMapping{HostPort: pm.Source, ContainerPort: pm.Destination})
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
			_, err = c.ContainerV1().Start(ctx, cname)
			if err != nil {
				logrus.Fatal(err)
			}

			logrus.Infof("requested to run container %s", cname)

			if viper.GetBool("wait") {
				err = c.EventV1().Wait(events.EventType_ContainerStarted, cname)
				if err != nil {
					logrus.Fatal(err)
				}
			}

			logrus.Infof("successfully started container %s", cname)
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
