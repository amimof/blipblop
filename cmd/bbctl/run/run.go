package run

import (
	"context"
	"log"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/amimof/blipblop/api/services/containers/v1"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/amimof/blipblop/pkg/networking"
	"github.com/sirupsen/logrus"
)

var (
	image string
	ports []string
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
		Run: func(_ *cobra.Command, args []string) {
			cname := args[0]
			ctx := context.Background()

			// Setup ports
			var cports []*containers.Port
			for _, p := range ports {

				pm, err := networking.ParsePorts(p)
				if err != nil {
					log.Fatal(err)
				}
				cports = append(cports, &containers.Port{Hostport: pm.Source, Containerport: pm.Destination})
			}

			server := viper.GetString("server")

			// Setup our client
			c, err := client.New(server)
			if err != nil {
				logrus.Fatal(err)
			}

			err = c.ContainerV1().CreateContainer(ctx, &containers.Container{
				Name: cname,
				Config: &containers.Config{
					Image: image,
					Ports: cports,
				},
			})
			if err != nil {
				log.Fatal(err)
			}
			err = c.ContainerV1().StartContainer(ctx, cname)
			if err != nil {
				log.Fatal(err)
			}
			log.Printf("request to run container %s successful", cname)
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
		"Forward a local port to the container",
	)
	if err := runCmd.MarkFlagRequired("image"); err != nil {
		log.Fatal(err)
	}
	return runCmd
}
