// Package edit provides ability to edit resources from the server
package edit

import (
	"github.com/amimof/blipblop/pkg/client"
	"github.com/spf13/cobra"
)

var (
	wait               bool
	waitTimeoutSeconds uint64
	resourceLabels     map[string]string
)

var editCmd *cobra.Command

func NewCmdEdit(cfg *client.Config) *cobra.Command {
	editCmd = &cobra.Command{
		Use:     "edit",
		Short:   "Edit a resource",
		Long:    "Edit a resource",
		Example: `bbctl edit set`,
		Args:    cobra.ExactArgs(1),
	}

	editCmd.PersistentFlags().BoolVarP(&wait, "wait", "w", true, "Wait for command to finish")
	editCmd.PersistentFlags().Uint64VarP(&waitTimeoutSeconds, "timeout", "", 30, "How long in seconds to wait for container to start before giving up")
	editCmd.PersistentFlags().StringToStringVarP(&resourceLabels, "labels", "l", map[string]string{}, "Resource labels as key value pair")
	editCmd.AddCommand(NewCmdEditContainer(cfg))

	return editCmd
}
