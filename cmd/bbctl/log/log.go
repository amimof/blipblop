// Package log provides ability to log resources from the server
package log

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/amimof/blipblop/api/services/logs/v1"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
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

			req := logs.TailLogRequest{
				NodeId:      ctr.GetStatus().GetNode().GetValue(),
				ContainerId: ctr.GetMeta().GetName(),
				Watch:       true,
			}

			stream, err := c.LogV1().TailLogs(ctx, &req)
			if err != nil {
				logrus.Fatalf("error setting up log stream: %v", err)
			}

			go func() {
				for {
					entry, err := stream.Recv()
					if err != nil {
						if errors.Is(err, io.EOF) || status.Code(err) == codes.Canceled {
							return
						}
						logrus.Errorf("error on stream: %v", err)
						return
					}
					fmt.Printf("%s %s/%s: %s\n",
						entry.GetTimestamp().AsTime().Format(time.RFC3339),
						entry.GetNodeId(),
						entry.GetContainerId(),
						entry.GetLine(),
					)
				}
			}()

			<-exit

			return nil
		},
	}

	return logCmd
}
