// Package upgrade provides ability to upgrade nodes
package upgrade

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"

	"github.com/amimof/blipblop/pkg/client"
	"github.com/amimof/blipblop/pkg/cmdutil"
)

var (
	selector []string
	version  string
)

func NewCmdUpgrade() *cobra.Command {
	var cfg client.Config
	upgradeCmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade nodes",
		Long:  "Upgrade multiple node binaries by name or label selector",
		Example: `

# Upgrade a single node by name to latest version
bbctl upgrade prod-node-01

# Upgrade two nodes by name to specific version
bbctl upgrade node1 node2 --version v0.0.8
	
# Upgrade all nodes with label blipblop/arch==amd64 to latest version
bbctl upgrade -l blipblop/arch=amd64
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

			tracer := otel.Tracer("bbctl")
			ctx, span := tracer.Start(ctx, "bbctl.stop.container")
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

			// Parse the version from flag
			ver, err := cmdutil.ParseVersion(version)
			if err != nil {
				logrus.Fatalf("error parsing version %s: %v", version, err)
			}

			// Upgrade nodes by name
			if len(args) > 0 {
				for _, n := range args {
					err = c.NodeV1().Upgrade(ctx, n, ver)
					if err != nil {
						logrus.Fatalf("error requesting node %s to upgrade: %v", n, err)
					}
					logrus.Infof("requested node %s to upgrade to version %s", n, ver)
				}
			}

			// Upgrade nodes by label selector
			if selector != nil {

				labelMap, err := parseSelectors(selector)
				if err != nil {
					logrus.Fatalf("error parsing label selectors: %v", err)
				}

				err = c.NodeV1().UpgradeAll(ctx, labelMap, ver)
				if err != nil {
					logrus.Fatalf("error requesting nodes to upgrade: %v", err)
				}
				logrus.Infof("requested nodes to upgrade to version %s", ver)
			}
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
