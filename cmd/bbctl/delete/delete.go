package delete

import (
	"context"
	"log"

	"github.com/spf13/cobra"

	"github.com/amimof/blipblop/pkg/client"
)

var (
	image string
)

func NewCmdDelete(c *client.ClientSet) *cobra.Command {
	runCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a container",
		Long:  "Delete a container",
		Example: `
# Delete a prometheus container
bbctl delete prometheus`,
		Args: cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			ctx := context.Background()
			cname := args[0]
			ctr, err := c.ContainerV1().GetContainer(ctx, cname)
			if err != nil {
				log.Fatal(err)
			}
			log.Println(ctr)
			err = c.ContainerV1().DeleteContainer(context.Background(), ctr.Name)
			if err != nil {
				log.Fatal(err)
			}
		},
	}
	return runCmd
}
