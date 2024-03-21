package run

import (
	"context"
	"log"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/amimof/blipblop/api/services/containers/v1"
	"github.com/amimof/blipblop/pkg/client"
)

var (
	image string
	ports []string
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

			// Setup ports
			var cports []*containers.Port
			for _, p := range ports {
				sport := strings.Split(p, "")
				source, err := strconv.Atoi(sport[0])
				if err != nil {
					log.Fatal(err)
				}
				dest, err := strconv.Atoi(sport[1])
				if err != nil {
					log.Fatal(err)
				}
				cports = append(cports, &containers.Port{Hostport: int32(source), Containerport: int32(dest)})
			}

			err := c.ContainerV1().CreateContainer(ctx, &containers.Container{
				Name: cname,
				Config: &containers.Config{
					Image: image,
					Ports: cports,
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
