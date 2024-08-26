package get

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"text/tabwriter"

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
			wr := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', tabwriter.AlignRight)

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
				fmt.Fprintln(wr, fmt.Sprintf("%s\t%s\t%s\t%s\t", "NAME", "REVISION", "STATE", "NODE"))
				for _, c := range containers {
					fmt.Fprintln(wr, fmt.Sprintf("%s\t%d\t%s\t%s\t", c.GetName(), c.GetRevision(), c.GetStatus().GetState(), c.GetStatus().GetNode()))
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
