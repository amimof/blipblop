package get

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"text/tabwriter"
	"time"

	"github.com/amimof/blipblop/pkg/client"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

func NewCmdGetContainer() *cobra.Command {
	runCmd := &cobra.Command{
		Use:     "container",
		Short:   "Get a container",
		Long:    "Get a container",
		Example: `bbctl get container`,
		Args:    cobra.MaximumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				return err
			}
			return nil
		},
		Run: func(_ *cobra.Command, args []string) {
			server := viper.GetString("server")
			ctx := context.Background()

			// Setup writer
			wr := tabwriter.NewWriter(os.Stdout, 8, 8, 8, '\t', tabwriter.AlignRight)

			// Setup our client
			c, err := client.New(ctx, server)
			if err != nil {
				logrus.Fatal(err)
			}
			if len(args) == 0 {
				containers, err := c.ContainerV1().ListContainers(context.Background())
				if err != nil {
					log.Fatal(err)
				}
				fmt.Fprintf(wr, "%s\t%s\t%s\t%s\t%s\t%s\n", "NAME", "REVISION", "PHASE", "CONDITION", "NODE", "AGE")
				for _, c := range containers {
					fmt.Fprintf(wr, "%s\t%d\t%s\t%s\t%s\t%s\n", c.GetName(), c.GetRevision(), c.GetStatus().GetPhase(), c.GetStatus().GetCondition(), c.GetStatus().GetNode(), time.Since(c.GetCreated().AsTime()).Round(1*time.Second))
				}
			}

			wr.Flush()

			if len(args) == 1 {
				var b bytes.Buffer
				enc := yaml.NewEncoder(&b)
				enc.SetIndent(2)
				cname := args[0]
				container, err := c.ContainerV1().GetContainer(context.Background(), cname)
				if err != nil {
					log.Fatal(err)
				}
				err = enc.Encode(&container)
				if err != nil {
					log.Fatal(err)
				}
				fmt.Printf("%s\n", b.String())
			}
		},
	}

	return runCmd
}
