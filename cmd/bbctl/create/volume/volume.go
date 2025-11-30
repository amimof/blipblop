// Package create provides ability to create resources from the server
package volume

import (
	"github.com/amimof/blipblop/pkg/client"
	"github.com/spf13/cobra"
)

var createCmd *cobra.Command

func NewCmdCreateVolume(cfg *client.Config) *cobra.Command {
	createCmd = &cobra.Command{
		Use:     "volume",
		Short:   "Create a volume",
		Long:    "Create a volume",
		Example: `bbctl create set`,
		Args:    cobra.ExactArgs(1),
	}

	createCmd.AddCommand(NewCmdCreateHostLocalVolume(cfg))
	createCmd.AddCommand(NewCmdCreateTemplateVolume(cfg))

	return createCmd
}
