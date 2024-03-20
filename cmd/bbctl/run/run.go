package run

import (
	"github.com/spf13/cobra"

	"github.com/amimof/blipblop/pkg/client"
)

func NewCmdRun(c *client.ClientSet) *cobra.Command {
	versionCmd := &cobra.Command{
		Use:     "run",
		Short:   "Run a container",
		Example: `bbctl run NAME --image=prom/prometheus:latest`,
		Run: func(_ *cobra.Command, _ []string) {

		},
	}
	return versionCmd
}
