package get

import (
	"github.com/amimof/blipblop/pkg/client"
	"github.com/spf13/cobra"
)

func NewCmdGet(cfg *client.Config) *cobra.Command {
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

	getCmd.AddCommand(NewCmdGetEvent(cfg))
	getCmd.AddCommand(NewCmdGetNode(cfg))
	getCmd.AddCommand(NewCmdGetContainer(cfg))
	getCmd.AddCommand(NewCmdGetContainerSet(cfg))

	return getCmd
}
