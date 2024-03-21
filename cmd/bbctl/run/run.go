package run

import (
	"context"
	"log"

	"github.com/spf13/cobra"

	"github.com/amimof/blipblop/api/services/containers/v1"
	"github.com/amimof/blipblop/pkg/client"
)

var (
	image string
)

func NewCmdRun(c *client.ClientSet) *cobra.Command {
	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Run a container",
		Long:  "Run a container. The run command required an image to be provided. The image must be in the format: registry/repo/image:tag",
		Example: `
# Run a prometheus container
bbctl run prometheus --image=docker.io/prom/prometheus:latest`,
		Args: cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			cname := args[0]
			ctx := context.Background()
			err := c.ContainerV1().CreateContainer(ctx, &containers.Container{
				Name: cname,
				Config: &containers.Config{
					Image: image,
				},
			})
			if err != nil {
				log.Fatal(err)
			}
			err = c.ContainerV1().RunContainer(ctx, cname)
			if err != nil {
				log.Fatal(err)
			}
			log.Printf("Container %s created", cname)
		},
	}
	runCmd.Flags().StringVarP(
		&image,
		"image",
		"i",
		"",
		"Container image to run, must include the registry host",
	)
	if err := runCmd.MarkFlagRequired("image"); err != nil {
		log.Fatal(err)
	}
	return runCmd
}
