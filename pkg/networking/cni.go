package networking

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"path"
	"path/filepath"
	"strings"

	gocni "github.com/containerd/go-cni"
	"github.com/sirupsen/logrus"
)

const (
	// CNIBinDir describes the directory where the CNI binaries are stored
	CNIBinDir = "/opt/cni/bin"

	// CNIConfDir describes the directory where the CNI plugin's configuration is stored
	CNIConfDir = "/etc/cni/net.d"

	// NetNSPathFmt gives the path to the a process network namespace, given the pid
	NetNSPathFmt = "/proc/%d/ns/net"

	// CNIDataDir is the directory CNI stores allocated IP for containers
	CNIDataDir = "/var/run/cni"

	// defaultCNIConfFilename is the vanity filename of default CNI configuration file
	defaultCNIConfFilename = "10-blipblop.conflist"

	// defaultNetworkName names the "docker-bridge"-like CNI plugin-chain installed when no other CNI configuration is present.
	// This value appears in iptables comments created by CNI.
	defaultNetworkName = "blipblop-cni-bridge"

	// defaultBridgeName is the default bridge device name used in the defaultCNIConf
	defaultBridgeName = "blipblop0"

	// defaultSubnet is the default subnet used in the defaultCNIConf -- this value is set to not collide with common container networking subnets:
	defaultSubnet = "10.69.0.0/16"

	// defaultSubnetGw is the gateway used for the subnet as defined in defaultSubnet
	defaultSubnetGw = "10.69.0.1"

	// defaultIfPrefix is the interface name to be created in the container
	defaultIfPrefix = "eth"
)

//"dataDir": "%s",

var defaultCNIConf = fmt.Sprintf(`
{
    "cniVersion": "0.4.0",
    "name": "%s",
    "plugins": [
      {
        "type": "bridge",
        "bridge": "%s",
        "isGateway": true,
        "ipMasq": true,
				"hairpinMode": true,
        "ipam": {
            "type": "host-local",
						"routes": [{ "dst": "0.0.0.0/0" }],
						"dataDir": "%s",
						"ranges": [[{
							"subnet": "%s",
							"gateway": "%s"
						}]]
        }
      },
      {
        "type": "portmap",
        "capabilities": {"portMappings": true}
      },
      {
        "type": "firewall"
      }
    ]
}
`, defaultNetworkName, defaultBridgeName, CNIDataDir, defaultSubnet, defaultSubnetGw)

// netID generates the network IF based on task name and task PID
func netID(id string, pid uint32) string {
	return fmt.Sprintf("%s-%d", id, pid)
}

// netNamespace generates the namespace path based on task PID.
func netNamespace(pid uint32) string {
	return fmt.Sprintf(NetNSPathFmt, pid)
}

// InitNetwork ...
func InitNetwork() (gocni.CNI, error) {
	logrus.Printf("Writing CNI network configuration to %s/%s", CNIConfDir, defaultCNIConfFilename)
	// Create directories
	_, err := os.Stat(CNIConfDir)
	if !os.IsNotExist(err) {
		if err := os.MkdirAll(CNIConfDir, 0755); err != nil {
			return nil, fmt.Errorf("cannot create directory: '%s'. error: %w", CNIConfDir, err)
		}
	}

	// Create network config file
	netconfig := path.Join(CNIConfDir, defaultCNIConfFilename)
	if err := os.WriteFile(netconfig, []byte(defaultCNIConf), 0644); err != nil {
		return nil, fmt.Errorf("cannot write network config: '%s'. error: %w", defaultCNIConfFilename, err)
	}

	// Initialize CNI library
	cni, err := gocni.New(
		gocni.WithMinNetworkCount(2),
		gocni.WithPluginConfDir(CNIConfDir),
		gocni.WithPluginDir([]string{CNIBinDir}),
		gocni.WithInterfacePrefix(defaultIfPrefix),
	)
	if err != nil {
		return nil, fmt.Errorf("error initializing cni: %w", err)
	}

	// Load the cni configuration
	if err := cni.Load(gocni.WithLoNetwork, gocni.WithConfListFile(filepath.Join(CNIConfDir, defaultCNIConfFilename))); err != nil {
		return nil, fmt.Errorf("failed to load cni configuration: %w", err)
	}

	return cni, nil
}

// CreateCNINetwork creates a CNI network interface and attaches it to the context
func CreateCNINetwork(ctx context.Context, cni gocni.CNI, id string, pid uint32, opts ...gocni.NamespaceOpts) (*gocni.Result, error) {
	i := netID(id, pid)
	n := netNamespace(pid)
	result, err := cni.Setup(ctx, i, n, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to setup network for task %q: %w", id, err)
	}

	return result, nil
}

func DeleteCNINetwork(ctx context.Context, cni gocni.CNI, id string, pid uint32, opts ...gocni.NamespaceOpts) error {
	i := netID(id, pid)
	n := netNamespace(pid)
	err := cni.Remove(ctx, i, n, opts...)
	if err != nil {
		return fmt.Errorf("failed to teardown network for task: %q, %v", id, err)
	}
	return nil
}

func GetIPAddress(id string) (net.IP, error) {
	netdir := path.Join(CNIDataDir, defaultNetworkName)
	fileinfos, err := os.ReadDir(netdir)
	if err != nil {
		return nil, err
	}
	for _, fileinfo := range fileinfos {
		if fileinfo.Name() == "lock" {
			continue
		}
		f, err := os.Open(path.Join(netdir, fileinfo.Name()))
		if err != nil {
			return nil, err
		}
		defer f.Close()
		reader := bufio.NewReader(f)
		content, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		if strings.Contains(content, id) {
			return net.ParseIP(fileinfo.Name()), nil
		}
	}
	return nil, fmt.Errorf("couldn't find IP Address for %s", id)
}
