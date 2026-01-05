package delete

import (
	"context"
	"fmt"

	"github.com/amimof/voiyd/pkg/client"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
)

func NewCmdDeleteTask(cfg *client.Config) *cobra.Command {
	runCmd := &cobra.Command{
		Use:     "task",
		Short:   "Delete a task",
		Long:    "Delete a task",
		Example: `voiydctl delete task NAME`,
		Args:    cobra.MinimumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				return err
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			tracer := otel.Tracer("voiydctl")
			ctx, span := tracer.Start(ctx, "voiydctl.delete.task")
			defer span.End()

			// Setup client
			currentSrv, err := cfg.CurrentServer()
			if err != nil {
				logrus.Fatal(err)
			}
			c, err := client.New(currentSrv.Address, client.WithTLSConfigFromCfg(cfg))
			if err != nil {
				logrus.Fatalf("error setting up client: %v", err)
			}
			defer func() {
				if err := c.Close(); err != nil {
					logrus.Errorf("error closing client: %v", err)
				}
			}()

			for _, tname := range args {
				fmt.Printf("Requested to delete task %s\n", tname)
				err = c.TaskV1().Delete(ctx, tname)
				if err != nil {
					logrus.Fatal(err)
				}
			}
		},
	}

	return runCmd
}
