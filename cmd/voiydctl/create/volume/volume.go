// Package volume provides command line features to create volume resources
package volume

import (
	"github.com/amimof/voiyd/pkg/client"
	"github.com/spf13/cobra"
)

var createCmd *cobra.Command

func NewCmdCreateVolume(cfg *client.Config) *cobra.Command {
	createCmd = &cobra.Command{
		Use:     "volume",
		Short:   "Create a volume",
		Long:    "Create a volume",
		Example: `voiydctl create set`,
		Args:    cobra.ExactArgs(1),
	}

	createCmd.AddCommand(NewCmdCreateHostLocalVolume(cfg))
	createCmd.AddCommand(NewCmdCreateTemplateVolume(cfg))

	return createCmd
}
