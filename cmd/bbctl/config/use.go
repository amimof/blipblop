package config

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"

	"github.com/amimof/blipblop/pkg/client"
	"github.com/fatih/color"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

func NewCmdConfigUse() *cobra.Command {
	var cfg client.Config
	cmd := &cobra.Command{
		Use:   "use NAME",
		Short: "Switch to another server in bbctl client configuration",
		Long:  "Switch to another server in bbctl client configuration",
		Example: `
# Switch to server 'production'
bbctl config use production
`,
		Args: cobra.MaximumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
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
			var serverName string

			// Fuzzy finder if no server name is provided on cmd line
			if len(args) == 0 {
				s, err := fzfListServers(&cfg)
				if err != nil {
					logrus.Fatalf("error listing servers: %v", err)
				}
				serverName = s
				cfg.Current = serverName
			}

			// Use server name from args
			if len(args) > 0 {
				serverName = args[0]

				s, err := cfg.GetServer(serverName)
				if err != nil {
					logrus.Fatalf("error using server %s: %v", serverName, err)
				}

				cfg.Current = s.Name
			}

			b, err := yaml.Marshal(cfg)
			if err != nil {
				logrus.Fatalf("error marshal: %v", err)
			}

			err = os.WriteFile(viper.GetViper().ConfigFileUsed(), b, 0o666)
			if err != nil {
				logrus.Fatalf("error writing config file: %v", err)
			}

			logrus.Infof("Using server server %s", serverName)
		},
	}

	return cmd
}

// Fzf uses fzf to display a fuzzy finder in stdout.
// Returns a string containing the selected item in the fzf result
func fzf(in bytes.Buffer) (string, error) {
	// TODO: Explore if we can use preview in fzf
	cmd := exec.Command("/opt/homebrew/bin/fzf", "--ansi", "--height=~10")
	var out bytes.Buffer

	// var in bytes.Buffer
	cmd.Stdin = &in
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			return "", err
		}
	}

	res := strings.TrimSpace(out.String())
	if res == "" {
		return "", errors.New("nothing selected")
	}

	return res, nil
}

func fzfListServers(cfg *client.Config) (string, error) {
	var in bytes.Buffer

	// Write each context to stdin
	for _, k := range cfg.Servers {
		if k.Name == cfg.Current {
			in.WriteString(color.GreenString(k.Name))
		} else {
			in.WriteString(k.Name)
		}
		in.WriteString("\n")
	}

	return fzf(in)
}
