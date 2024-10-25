package delete

import (
	"github.com/amimof/blipblop/pkg/client"
	"github.com/spf13/cobra"
)

var (
	wait               bool
	waitTimeoutSeconds uint64
)

func NewCmdDelete(cfg *client.Config) *cobra.Command {
	deleteCmd := &cobra.Command{
		Use:     "delete",
		Short:   "Delete a resource",
		Long:    "Delete a resource",
		Example: `bbctl delete container`,
		Args:    cobra.ExactArgs(1),
	}

	deleteCmd.PersistentFlags().BoolVarP(&wait, "wait", "w", true, "Wait for command to finish")
	deleteCmd.PersistentFlags().Uint64VarP(&waitTimeoutSeconds, "timeout", "", 30, "How long in seconds to wait for container to stop before giving up")

	deleteCmd.AddCommand(NewCmdDeleteContainer(cfg))
	deleteCmd.AddCommand(NewCmdDeleteNode(cfg))
	deleteCmd.AddCommand(NewCmdDeleteContainerSet(cfg))

	return deleteCmd
}
