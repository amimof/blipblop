package create

import (
	"github.com/amimof/blipblop/pkg/client"
	"github.com/spf13/cobra"
)

var (
	wait               bool
	waitTimeoutSeconds uint64
)

func NewCmdCreate(cfg *client.Config) *cobra.Command {
	createCmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a resource",
		Long:    "Create a resource",
		Example: `bbctl create container`,
		Args:    cobra.ExactArgs(1),
	}

	createCmd.PersistentFlags().BoolVarP(&wait, "wait", "w", true, "Wait for command to finish")
	createCmd.PersistentFlags().Uint64VarP(&waitTimeoutSeconds, "timeout", "", 30, "How long in seconds to wait for container to stop before giving up")

	createCmd.AddCommand(NewCmdCreateSet(cfg))

	return createCmd
}
