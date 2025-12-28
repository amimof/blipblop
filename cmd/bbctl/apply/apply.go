// Package apply provides ability to apply resources to the server
package apply

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.yaml.in/yaml/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	containersv1 "github.com/amimof/blipblop/api/services/containers/v1"
	containersetsv1 "github.com/amimof/blipblop/api/services/containersets/v1"
	nodesv1 "github.com/amimof/blipblop/api/services/nodes/v1"
	volumesv1 "github.com/amimof/blipblop/api/services/volumes/v1"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/amimof/blipblop/pkg/cmdutil"
	"github.com/amimof/blipblop/services/container"
	"github.com/amimof/blipblop/services/containerset"
	"github.com/amimof/blipblop/services/node"
	"github.com/amimof/blipblop/services/volume"
)

var file string

var applyCmd *cobra.Command

type genericHeader struct {
	Version string `yaml:"version" json:"version"`
}

func detectVersion(doc []byte) (string, error) {
	var h genericHeader
	if err := yaml.Unmarshal(doc, &h); err != nil {
		return "", fmt.Errorf("decode version: %w", err)
	}

	if h.Version == "" {
		return "", fmt.Errorf("missing version field")
	}

	parts := strings.SplitN(h.Version, "/", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid version %q", h.Version)
	}
	return h.Version, nil
}

func NewCmdApply(cfg *client.Config) *cobra.Command {
	applyCmd = &cobra.Command{
		Use:   "apply",
		Short: "Apply resources from a file",
		Long:  "Apply updates or creates resources defined in the provided yaml/json file",
		Example: `
# Apply resources defined as yaml in a file
bbctl apply -f resources.yaml
`,
		Args: cobra.ExactArgs(0),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				return err
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()

			// Read file content
			data, err := os.ReadFile(file)
			if err != nil {
				logrus.Fatalf("error reading file: %v", err)
			}

			// Setup client
			c, err := client.New(cfg.CurrentServer().Address, client.WithTLSConfigFromCfg(cfg))
			if err != nil {
				logrus.Fatalf("error setting up client: %v", err)
			}
			defer func() {
				if err := c.Close(); err != nil {
					logrus.Fatalf("error closing client connection: %v", err)
				}
			}()

			codec := cmdutil.NewYamlCodec()
			dec := yaml.NewDecoder(strings.NewReader(string(data)))
			idx := 0

			for {

				var raw any
				if err := dec.Decode(&raw); err != nil {
					if err == io.EOF {
						break
					}
					logrus.Fatalf("error decoding document %d: %v", idx, err)
				}

				idx++
				if raw == nil {
					continue
				}

				// Re-encode that doc back to YAML bytes
				docBytes, err := yaml.Marshal(raw)
				if err != nil {
					logrus.Fatalf("error marshal document %d: %v", idx, err)
				}

				// Detect kind by version
				v, err := detectVersion(docBytes)
				if err != nil {
					logrus.Fatalf("error detecting version %d: %v", idx, err)
				}

				switch v {
				case node.Version:
					var n nodesv1.Node
					if err := codec.Deserialize(docBytes, &n); err != nil {
						logrus.Fatalf("error deserializing %s %d: %v", v, idx, err)
					}
					if err := applyNode(ctx, c, &n); err != nil {
						logrus.Fatalf("error applying %s %d: %v", v, idx, err)
					}
				case container.Version:
					var ctr containersv1.Container
					if err := codec.Deserialize(docBytes, &ctr); err != nil {
						logrus.Fatalf("error deserializing %s %d: %v", v, idx, err)
					}
					if err := applyContainer(ctx, c, &ctr); err != nil {
						logrus.Fatalf("error applying %s %d: %v", v, idx, err)
					}
				case volume.Version:
					var vol volumesv1.Volume
					if err := codec.Deserialize(docBytes, &vol); err != nil {
						logrus.Fatalf("error deserializing %s %d: %v", v, idx, err)
					}
					if err := applyVolume(ctx, c, &vol); err != nil {
						logrus.Fatalf("error applying %s %d: %v", v, idx, err)
					}
				case containerset.Version:
					var cs containersetsv1.ContainerSet
					if err := codec.Deserialize(docBytes, &cs); err != nil {
						logrus.Fatalf("error deserializing %s %d: %v", v, idx, err)
					}
					if err := applyContainerSet(ctx, c, &cs); err != nil {
						logrus.Fatalf("error applying %s %d: %v", v, idx, err)
					}
				default:
					logrus.Fatalf("document %d: unsupported version %s", idx, v)
				}
			}
			logrus.Infof("applied %d resource(s) from %s", idx, file)
		},
	}
	applyCmd.Flags().StringVarP(&file, "file", "f", "", "Path to file with resource definitions")
	if err := applyCmd.MarkFlagRequired("file"); err != nil {
		logrus.Fatal(err)
	}

	return applyCmd
}

func applyNode(ctx context.Context, c *client.ClientSet, n *nodesv1.Node) error {
	name := n.GetMeta().GetName()
	if name == "" {
		return fmt.Errorf("node.meta.name is required")
	}

	// Try to get existing
	_, err := c.NodeV1().Get(ctx, name)
	if status.Code(err) == codes.NotFound {
		if n.Version == "" {
			n.Version = node.Version
		}
		logrus.Infof("created %s: %s", n.Version, n.GetMeta().GetName())
		return c.NodeV1().Create(ctx, n)
	}
	if err != nil {
		return err
	}

	// Update if already exists
	logrus.Infof("updated %s: %s", n.Version, n.GetMeta().GetName())
	return c.NodeV1().Update(ctx, name, n)
}

func applyContainer(ctx context.Context, c *client.ClientSet, ctr *containersv1.Container) error {
	name := ctr.GetMeta().GetName()
	if name == "" {
		return fmt.Errorf("container.meta.name is required")
	}

	// Try to get existing
	_, err := c.ContainerV1().Get(ctx, name)
	if status.Code(err) == codes.NotFound {
		if ctr.Version == "" {
			ctr.Version = container.Version
		}
		logrus.Infof("created %s: %s", ctr.Version, ctr.GetMeta().GetName())
		return c.ContainerV1().Create(ctx, ctr)
	}
	if err != nil {
		return err
	}

	// Update if already exists
	logrus.Infof("updated %s: %s", ctr.Version, ctr.GetMeta().GetName())
	return c.ContainerV1().Update(ctx, name, ctr)
}

func applyVolume(ctx context.Context, c *client.ClientSet, v *volumesv1.Volume) error {
	name := v.GetMeta().GetName()
	if name == "" {
		return fmt.Errorf("volume.meta.name is required")
	}

	// Try to get existing
	_, err := c.VolumeV1().Get(ctx, name)
	if status.Code(err) == codes.NotFound {
		if v.Version == "" {
			v.Version = volume.Version
		}
		logrus.Infof("created %s: %s", v.Version, v.GetMeta().GetName())
		return c.VolumeV1().Create(ctx, v)
	}
	if err != nil {
		return err
	}

	// Update if already exists
	logrus.Infof("updated %s: %s", v.Version, v.GetMeta().GetName())
	return c.VolumeV1().Update(ctx, name, v)
}

func applyContainerSet(ctx context.Context, c *client.ClientSet, cs *containersetsv1.ContainerSet) error {
	name := cs.GetMeta().GetName()
	if name == "" {
		return fmt.Errorf("containerset.meta.name is required")
	}

	// Try to get existing
	_, err := c.ContainerSetV1().Get(ctx, name)
	if status.Code(err) == codes.NotFound {
		if cs.Version == "" {
			cs.Version = containerset.Version
		}
		logrus.Infof("created %s: %s", cs.Version, cs.GetMeta().GetName())
		return c.ContainerSetV1().Create(ctx, cs)
	}
	if err != nil {
		return err
	}

	// Update if already exists
	logrus.Infof("updated %s: %s", cs.Version, cs.GetMeta().GetName())
	return c.ContainerSetV1().Create(ctx, cs)
}
