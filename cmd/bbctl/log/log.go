// Package log provides ability to log resources from the server
package log

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/amimof/blipblop/api/services/logs/v1"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc/metadata"
)

func NewCmdLog(cfg *client.Config) *cobra.Command {
	logCmd := &cobra.Command{
		Use:     "log",
		Short:   "Read container logs",
		Long:    "Streams container logs from the node to stdout",
		Example: `bbctl log CONTAINER_NAME`,
		Args:    cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				return err
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			clientID := uuid.New().String()
			ctx, cancel := context.WithCancel(context.Background())
			ctx = metadata.AppendToOutgoingContext(ctx, "blipblop_client_id", clientID)
			defer cancel()

			// Setup client
			c, err := client.New(cfg.CurrentServer().Address, client.WithTLSConfigFromCfg(cfg))
			if err != nil {
				logrus.Fatalf("error setting up client: %v", err)
			}
			defer func() {
				if err := c.Close(); err != nil {
					logrus.Errorf("error closing client: %v", err)
				}
			}()

			// Setup signal handlers
			exit := make(chan os.Signal, 1)
			signal.Notify(exit, os.Interrupt, syscall.SIGTERM)

			// We need to know which node the container is scheduled on
			containerID := args[0]
			ctr, err := c.ContainerV1().Get(ctx, containerID)
			if err != nil {
				return err
			}

			// Setup channels for the stream
			logChan := make(chan *logs.SubscribeResponse, 10)
			errChan := make(chan error, 1)

			// Handle incoming logs and errors
			go func() {
				for {
					select {
					case <-ctx.Done():
						return
					case e := <-errChan:
						fmt.Printf("error: %v", e)
					case l := <-logChan:
						fmt.Printf("%s: %s\n", l.GetLog().GetTimestamp(), l.GetLog().GetLogLine())
					}
				}
			}()

			// Start streaming
			go func() {
				for {
					select {
					case <-ctx.Done():
						return
					case e := <-errChan:
						fmt.Printf("error %v\n", e)
					default:
						err = c.LogV1().StreamLogs(ctx, &logs.SubscribeRequest{NodeId: ctr.GetStatus().GetNode().String(), ContainerId: ctr.GetMeta().GetName(), ClientId: clientID}, logChan, errChan)
						if err != nil {
							fmt.Println("Error streaming", err)
							return
						}
					}
				}
			}()

			<-exit
			cancel()
			close(logChan)
			return nil
		},
	}

	return logCmd
}
