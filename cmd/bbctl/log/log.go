package log

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/amimof/blipblop/api/services/logs/v1"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	wait               bool
	force              bool
	waitTimeoutSeconds uint64
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
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Setup client
			c, err := client.New(ctx, cfg.CurrentServer().Address, client.WithTLSConfigFromCfg(cfg))
			if err != nil {
				logrus.Fatalf("error setting up client: %v", err)
			}
			defer c.Close()

			// Setup signal handlers
			exit := make(chan os.Signal, 1)
			signal.Notify(exit, os.Interrupt, syscall.SIGTERM)

			// Start streaming
			logChan := make(chan *logs.LogResponse, 10)
			errChan := make(chan error, 1)

			go func() {
				err = c.LogV1().StreamLogs(ctx, "nginx2", logChan, errChan)
				if err != nil {
					fmt.Println("Error streaming", err)
					// return err
				}
			}()

			go func() {
				for {
					select {
					case <-ctx.Done():
						fmt.Println("Done watching, exit")
						return
					case e := <-errChan:
						fmt.Printf("error %v", e)
					case l := <-logChan:
						fmt.Printf("%s", l.LogLine)
					}
				}
			}()

			<-exit

			cancel()
			return nil
		},
	}

	// logCmd.PersistentFlags().BoolVarP(&wait, "wait", "w", true, "Wait for command to finish")
	// logCmd.PersistentFlags().BoolVar(&force, "force", false, "Attempt forceful shutdown of the continaner")
	// logCmd.PersistentFlags().Uint64VarP(&waitTimeoutSeconds, "timeout", "", 30, "How long in seconds to wait for container to log before giving up")

	// logCmd.AddCommand(NewCmdStopContainer(cfg))

	return logCmd
}
