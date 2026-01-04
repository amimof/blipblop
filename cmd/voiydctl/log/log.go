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

	"github.com/amimof/voiyd/api/services/logs/v1"
	"github.com/amimof/voiyd/pkg/client"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func NewCmdLog() *cobra.Command {
	var cfg client.Config
	logCmd := &cobra.Command{
		Use:     "log",
		Short:   "Read container logs",
		Long:    "Streams container logs from the node to stdout",
		Example: `voiydctl log CONTAINER_NAME`,
		Args:    cobra.ExactArgs(1),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				return err
			}
			if err := viper.ReadInConfig(); err != nil {
				logrus.Fatalf("error reading config: %v", err)
			}
			if err := viper.Unmarshal(&cfg); err != nil {
				logrus.Fatalf("error decoding config into struct: %v", err)
			}
			if err := cfg.Validate(); err != nil {
				logrus.Fatal(err)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			clientID := uuid.New().String()
			ctx, cancel := context.WithCancel(context.Background())
			ctx = metadata.AppendToOutgoingContext(ctx, "voiyd_client_id", clientID)
			defer cancel()

			// Setup client
			currentSrv, err := cfg.CurrentServer()
			if err != nil {
				logrus.Fatal(err)
			}
			c, err := client.New(currentSrv.Address, client.WithTLSConfigFromCfg(&cfg))
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
				streamCtx := stream.Context()
				for {
					select {
					case <-streamCtx.Done():
						return
					default:
						entry, err := stream.Recv()
						if err != nil {
							if errors.Is(err, io.EOF) || status.Code(err) == codes.Canceled {
								logrus.Errorf("stream canceled by server: %v", err)
								os.Exit(1)
								return
							}
							logrus.Errorf("error on stream: %v", err)
							os.Exit(1)
							return
						}
						fmt.Printf("%s %s/%s: %s\n",
							entry.GetTimestamp().AsTime().Format(time.RFC3339),
							entry.GetNodeId(),
							entry.GetContainerId(),
							entry.GetLine(),
						)
					}
				}
			}()

			<-exit

			return nil
		},
	}

	return logCmd
}
