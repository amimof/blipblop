// Package watch provides ability to watch resources
package watch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"

	"github.com/amimof/voiyd/pkg/client"
	"github.com/amimof/voiyd/pkg/cmdutil"
	"github.com/amimof/voiyd/pkg/events"

	leasesv1 "github.com/amimof/voiyd/api/services/leases/v1"
	tasksv1 "github.com/amimof/voiyd/api/services/tasks/v1"
)

var (
	selector []string
	version  string
)

func NewCmdWatch() *cobra.Command {
	var cfg client.Config
	upgradeCmd := &cobra.Command{
		Use:   "watch",
		Short: "Watch resources",
		Long:  "Watch resources in the cluster by subscribing to events",
		Example: `

# Watch task events 
voiydctl watch tasks

# Watch events for a specific task
voiydctl watch tasks my-task

# Watch events for nodes that matches a selector
voiydctl watch nodes --selector voiyd.io/arch=arm64
`,
		Args: cobra.MinimumNArgs(0),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				return err
			}

			hasSelector := selector != nil
			hasArgs := len(args) > 0

			if hasSelector == hasArgs {
				return errors.New("must only provide either args or --label-selector flag")
			}

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
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Second*30)
			defer cancel()

			tracer := otel.Tracer("voiydctl")
			ctx, span := tracer.Start(ctx, "voiydctl.stop.container")
			defer span.End()

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

			// 2. Setup signal handling (Ctrl+C to exit)
			exit := make(chan os.Signal, 1)
			signal.Notify(exit, os.Interrupt, syscall.SIGTERM)

			// Create watcher
			dashCtx, dashCancel := context.WithCancel(context.Background())
			defer dashCancel()

			watcher := cmdutil.NewDashboard(
				args,
				cmdutil.WithWriter(os.Stdout),
				cmdutil.WithDefaultText("Loading events..."),
				cmdutil.WithHeader(`{{"NAME"}}|20|{{ "Node"}}|20|{{"Task"}}|20|{{ "TTL" }}|20|{{ "AGE" }}|10|{{ "ExpiresIn" }}`),
				cmdutil.WithFormat("{{ .Name | FgYellow }}|20|{{ .Metadata.NodeID}}|20|{{ .Metadata.TaskID }}|20|{{ .Metadata.TTL }}|20|{{ .Metadata.Age | age }}|10|{{ .Metadata.ExpiresIn | age }}"),
			)
			go watcher.Loop(dashCtx)

			// Subscribe to events
			eventChan, errChan := c.EventV1().Subscribe(dashCtx, events.ALL...)

			go func() {
				for {
					select {
					case <-exit:
						dashCancel()
					case <-errChan:
						dashCancel()
					case <-dashCtx.Done():
						return
					case event := <-eventChan:

						obj, err := event.GetObject().UnmarshalNew()
						if err != nil {
							panic(err)
						}

						switch obj.(type) {
						case *tasksv1.Task:
							var task tasksv1.Task
							err := event.GetObject().UnmarshalTo(&task)
							if err != nil {
								panic(err)
							}
							md := map[string]any{
								"TaskName": task.GetMeta().GetName(),
								"Phase":    task.GetStatus().GetPhase().GetValue(),
								// "pid":   task.GetStatus().GetPid().GetValue(),
								"Node": task.GetStatus().GetNode().GetValue(),
							}
							watcher.SetMetadata(0, md)
							watcher.UpdateText(0, task.GetMeta().GetUid())
						case *leasesv1.Lease:
							var lease leasesv1.Lease
							err := event.GetObject().UnmarshalTo(&lease)
							if err != nil {
								panic(err)
							}
							md := map[string]any{
								"Age":       lease.GetConfig().GetAcquiredAt().AsTime(),
								"ExpiresIn": lease.GetConfig().GetExpiresAt().AsTime(),
								"TaskID":    lease.GetConfig().GetTaskId(),
								"NodeID":    lease.GetConfig().GetNodeId(),
								"TTL":       time.Duration(time.Second * time.Duration(lease.GetConfig().GetTtlSeconds())).String(),
							}
							watcher.SetMetadata(0, md)
							watcher.UpdateText(0, lease.GetMeta().GetUid())
						}
					}
				}
			}()
			watcher.Wait()
			close(eventChan)
		},
	}

	upgradeCmd.PersistentFlags().StringArrayVarP(&selector, "label-selector", "l", nil, "Label selector")
	upgradeCmd.PersistentFlags().StringVarP(&version, "target-version", "t", "latest", "Target version on upgrading")

	return upgradeCmd
}

func parseSelectors(inputs []string) (map[string]string, error) {
	out := make(map[string]string)
	for _, raw := range inputs {
		part := strings.TrimSpace(raw)
		if part == "" {
			continue
		}

		k, v, ok := strings.Cut(part, "=")
		if !ok {
			return nil, fmt.Errorf("invalid selector %q expected key=value", part)
		}

		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)

		out[k] = v

	}

	return out, nil
}
