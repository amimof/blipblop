package get

import (
	"bytes"
	"context"
	"fmt"
	"log"

	"github.com/amimof/blipblop/pkg/client"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func NewCmdGetContainer(c *client.ClientSet) *cobra.Command {
	runCmd := &cobra.Command{
		Use:     "container",
		Short:   "Get a container",
		Long:    "Get a container",
		Example: `bbctl get container`,
		Args:    cobra.MaximumNArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			if len(args) == 0 {
				containers, err := c.ContainerV1().ListContainers(context.Background())
				if err != nil {
					log.Fatal(err)
				}
				fmt.Printf("%s\t%s\t%s\t%s\n", "NAME", "REVISION", "STATE", "NODE")
				for _, c := range containers {
					fmt.Printf("%s\t%d\t%s\t%s\n", c.GetName(), c.GetRevision(), c.GetStatus().GetState(), c.GetStatus().GetNode())
				}
			}

			if len(args) == 1 {
				var b bytes.Buffer
				enc := yaml.NewEncoder(&b)
				enc.SetIndent(2)
				cname := args[0]
				container, err := c.ContainerV1().GetContainer(context.Background(), cname)
				if err != nil {
					log.Fatal(err)
				}
				err = enc.Encode(&container)
				if err != nil {
					log.Fatal(err)
				}
				fmt.Printf("%s\n", b.String())
			}
		},
	}

	return runCmd
}
