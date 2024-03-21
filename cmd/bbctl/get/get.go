package get

import (
	"github.com/spf13/cobra"

	"github.com/amimof/blipblop/pkg/client"
)

func NewCmdGet(c *client.ClientSet) *cobra.Command {
	getCmd := &cobra.Command{
		Use:   "get",
		Short: "Get a resource",
		Long:  "Get a resource",
		Example: `
# Get all containers
bbctl get containers

# Get a specific container
bbctl get container prometheus

# Get all nodes
bbctl get nodes
`,

		Args: cobra.ExactArgs(1),
	}

	getCmd.AddCommand(NewCmdGetNode(c))
	getCmd.AddCommand(NewCmdGetContainer(c))

	return getCmd
}
