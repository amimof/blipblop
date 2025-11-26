package delete

import (
	"github.com/amimof/blipblop/pkg/client"
	"github.com/spf13/cobra"
)

func NewCmdDelete(cfg *client.Config) *cobra.Command {
	deleteCmd := &cobra.Command{
		Use:     "delete",
		Short:   "Delete a resource",
		Long:    "Delete a resource",
		Example: `bbctl delete container`,
		Args:    cobra.ExactArgs(1),
	}

	deleteCmd.AddCommand(NewCmdDeleteContainer(cfg))
	deleteCmd.AddCommand(NewCmdDeleteNode(cfg))
	deleteCmd.AddCommand(NewCmdDeleteContainerSet(cfg))
	deleteCmd.AddCommand(NewCmdDeleteVolume(cfg))

	return deleteCmd
}
