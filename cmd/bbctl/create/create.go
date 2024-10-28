package create

import (
	"github.com/amimof/blipblop/pkg/client"
	"github.com/spf13/cobra"
)

var (
	wait               bool
	waitTimeoutSeconds uint64
	resourceLabels     map[string]string
)

var createCmd *cobra.Command

func NewCmdCreate(cfg *client.Config) *cobra.Command {
	createCmd = &cobra.Command{
		Use:     "create",
		Short:   "Create a resource",
		Long:    "Create a resource",
		Example: `bbctl create set`,
		Args:    cobra.ExactArgs(1),
	}

	createCmd.PersistentFlags().BoolVarP(&wait, "wait", "w", true, "Wait for command to finish")
	createCmd.PersistentFlags().Uint64VarP(&waitTimeoutSeconds, "timeout", "", 30, "How long in seconds to wait for container to start before giving up")
	createCmd.PersistentFlags().StringToStringVarP(&resourceLabels, "labels", "l", map[string]string{}, "Resource labels as key value pair")
	createCmd.AddCommand(NewCmdCreateSet(cfg))

	return createCmd
}
