package delete

import (
	"github.com/spf13/cobra"

	"github.com/amimof/blipblop/pkg/client"
)

func NewCmdDelete(c *client.ClientSet) *cobra.Command {
	deleteCmd := &cobra.Command{
		Use:     "delete",
		Short:   "Delete a resource",
		Long:    "Delete a resource",
		Example: `bbctl delete container`,
		Args:    cobra.ExactArgs(1),
	}

	deleteCmd.AddCommand(NewCmdDeleteContainer(c))

	return deleteCmd
}
