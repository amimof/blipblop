package start

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var wait bool

func NewCmdStartContainer() *cobra.Command {
	runCmd := &cobra.Command{
		Use:     "container",
		Short:   "Start a container",
		Long:    "Start a container",
		Example: `bbctl start container NAME`,
		Args:    cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				return err
			}
			return nil
		},
		Run: func(_ *cobra.Command, args []string) {
			server := viper.GetString("server")
			ctx := context.Background()

			// Setup our client
			c, err := client.New(server)
			if err != nil {
				logrus.Fatal(err)
			}

			cname := args[0]
			ctr, err := c.ContainerV1().GetContainer(ctx, cname)
			if err != nil {
				log.Fatal(err)
			}
			log.Printf("request to start container %s successful", ctr.GetMeta().GetName())
			evt := make(chan *events.Event)
			errChan := make(chan error)

			clientId := fmt.Sprintf("%s:%s", "bbctl", uuid.New())

			go func() {
				err = c.EventV1().Subscribe(ctx, clientId, evt, errChan)
				if err != nil {
					log.Printf("error subscribing to events")
				}
			}()

			err = c.ContainerV1().StartContainer(context.Background(), ctr.GetMeta().GetName())
			if err != nil {
				log.Fatal(err)
			}

			for {
				select {
				case e := <-evt:
					if e.Type == events.EventType_ContainerStarted && e.GetObjectId() == ctr.GetMeta().GetName() {
						log.Printf("successfully started container %s", ctr.GetMeta().GetName())
						return
					}
				case <-time.After(30 * time.Second):
					fmt.Println("timeout waiting for container to start")
					return
				}
			}
		},
	}

	runCmd.PersistentFlags().BoolVarP(
		&wait,
		"wait",
		"w",
		true,
		"Wait for command to finish",
	)
	return runCmd
}
