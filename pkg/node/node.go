package node

import (
	"fmt"
	"io"
	"os"
	"path"
	"runtime"

	"github.com/amimof/blipblop/api/services/nodes/v1"
	"github.com/amimof/blipblop/api/types/v1"
	"github.com/amimof/blipblop/pkg/labels"
	"github.com/amimof/blipblop/pkg/volume"
	"gopkg.in/yaml.v3"
)

// NodeVolumeManagers returns a list of drivers, each configured according to the provided Node spec.
// If no drivers are configured on the node then the list will be empty
func NodeVolumeManagers(n *nodes.Node) map[volume.VolumeType]volume.Driver {
	result := make(map[volume.VolumeType]volume.Driver)
	drivers := n.GetConfig().GetVolumeDrivers()
	if drivers == nil {
		return result
	}

	if drivers.GetHostLocal() != nil {
		result[volume.DriverTypeHostLocal] = volume.NewHostLocalManager(drivers.GetHostLocal().GetRootDir())
	}

	return result
}

func LoadNodeFromEnv(path string) (*nodes.Node, error) {
	// Attempt to load from the file
	n, err := loadFromFile(path)
	if err != nil {
		// If the file is missing, create a new node from the environment
		if os.IsNotExist(err) {
			n, err = NewNodeFromEnv()
			if err != nil {
				return nil, fmt.Errorf("failed to create node from environment: %w", err)
			}

			// Save the new node to the specified path
			err = createNodeFile(n, path)
			if err != nil {
				return nil, fmt.Errorf("failed to create node file: %w", err)
			}
		} else {
			// Return any other error from loadFromFile immediately
			return nil, fmt.Errorf("failed to load node from file: %w", err)
		}
	}
	return n, nil
}

func createNodeFile(n *nodes.Node, filePath string) error {
	b, err := yaml.Marshal(n)
	if err != nil {
		return err
	}

	p := path.Dir(filePath)
	err = os.MkdirAll(p, 0)
	if err != nil {
		return err
	}

	f, err := os.Create(filePath)
	if err != nil {
		return err
	}

	//nolint:errcheck
	defer f.Close()

	err = os.WriteFile(filePath, b, 0)
	if err != nil {
		return err
	}
	return nil
}

func loadFromFile(path string) (*nodes.Node, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	//nolint:errcheck
	defer f.Close()

	b, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	var node nodes.Node
	err = yaml.Unmarshal(b, &node)
	if err != nil {
		return nil, err
	}

	return &node, nil
}

// NewNodeFromEnv creates a new node from the current environment with the name s
func NewNodeFromEnv() (*nodes.Node, error) {
	// Hostname, arch and OS info
	arch := runtime.GOARCH
	oper := runtime.GOOS
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	// Construct labels
	l := labels.New()
	l.Set(labels.LabelPrefix("arch").String(), arch)
	l.Set(labels.LabelPrefix("os").String(), oper)

	// Construct node instance
	n := &nodes.Node{
		Meta: &types.Meta{
			Name:   hostname,
			Labels: l,
		},
	}
	return n, err
}
