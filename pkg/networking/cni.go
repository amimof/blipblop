package networking

import (
	"context"
	"fmt"
	"os"
	"path"

	gocni "github.com/containerd/go-cni"
	"go.opentelemetry.io/otel"
)

var tracer = otel.GetTracerProvider().Tracer("voiyd-node")

type Manager interface {
	Attach(ctx context.Context, containerID string, containerPID uint32, opts ...gocni.NamespaceOpts) (*gocni.Result, error)
	Detach(ctx context.Context, containerID string, containerPID uint32, opts ...gocni.NamespaceOpts) error
	Check(ctx context.Context, containerID string, containerPID uint32, opts ...gocni.NamespaceOpts) error
}

type CNIManager struct {
	cni       gocni.CNI
	cniOpts   []gocni.Opt
	cniNsOpts []gocni.NamespaceOpts

	// CNIBinDir describes the directory where the CNI binaries are stored
	CNIBinDir string

	// CNIConfDir describes the directory where the CNI plugin's configuration is stored
	CNIConfDir string

	// NetNSPathFmt gives the path to the a process network namespace, given the pid
	NetNSPathFmt string

	// CNIDataDir is the directory CNI stores allocated IP for containers
	CNIDataDir string

	// defaultCNIConfFilename is the vanity filename of default CNI configuration file
	DefaultCNIConfFilename string

	// defaultNetworkName names the "docker-bridge"-like CNI plugin-chain installed when no other CNI configuration is present.
	// This value appears in iptables comments created by CNI.
	DefaultNetworkName string

	// defaultBridgeName is the default bridge device name used in the defaultCNIConf
	DefaultBridgeName string

	// defaultSubnet is the default subnet used in the defaultCNIConf -- this value is set to not collide with common container networking subnets:
	DefaultSubnet string

	// defaultSubnetGw is the gateway used for the subnet as defined in defaultSubnet
	DefaultSubnetGw string

	// defaultIfPrefix is the interface name to be created in the container
	DefaultIfPrefix string

	// DefaultCNIConf is the CNI configuration file used by pluginsto setup networking
	DefaultCNIConf string
}

type UnimplementedManager struct{}

func (m *UnimplementedManager) Attach(ctx context.Context, id string, pid uint32, opts ...gocni.NamespaceOpts) (*gocni.Result, error) {
	return nil, fmt.Errorf("UnimplementedManager Attach() is unimplemented")
}

func (m *UnimplementedManager) Detach(ctx context.Context, id string, pid uint32, opts ...gocni.NamespaceOpts) error {
	return fmt.Errorf("UnimplementedManager Detach() is unimplemented")
}

func (m *UnimplementedManager) Check(ctx context.Context, id string, pid uint32, opts ...gocni.NamespaceOpts) error {
	return fmt.Errorf("UnimplementedManager Check() is unimplemented")
}

type CNIManagerOpts func(*CNIManager) error

func WithNamespaceOpts(opts ...gocni.NamespaceOpts) CNIManagerOpts {
	return func(m *CNIManager) error {
		m.cniNsOpts = append(m.cniNsOpts, opts...)
		return nil
	}
}

func WithCNIOpt(opt ...gocni.Opt) CNIManagerOpts {
	return func(m *CNIManager) error {
		m.cniOpts = append(m.cniOpts, opt...)
		return nil
	}
}

func WithCNIBinDir(s string) CNIManagerOpts {
	return func(m *CNIManager) error {
		m.CNIBinDir = s
		return nil
	}
}

func WithCNIConfDir(s string) CNIManagerOpts {
	return func(m *CNIManager) error {
		m.CNIConfDir = s
		return nil
	}
}

func WithNetNSPathFmt(s string) CNIManagerOpts {
	return func(m *CNIManager) error {
		m.NetNSPathFmt = s
		return nil
	}
}

func WithCNIDataDir(s string) CNIManagerOpts {
	return func(m *CNIManager) error {
		m.CNIDataDir = s
		return nil
	}
}

func WithDefaultCNIConfFilename(s string) CNIManagerOpts {
	return func(m *CNIManager) error {
		m.DefaultCNIConfFilename = s
		return nil
	}
}

func WithDefaultNetworkName(s string) CNIManagerOpts {
	return func(m *CNIManager) error {
		m.DefaultNetworkName = s
		return nil
	}
}

func WithDefaultBridgeName(s string) CNIManagerOpts {
	return func(m *CNIManager) error {
		m.DefaultBridgeName = s
		return nil
	}
}

func WithDefaultSubnet(s string) CNIManagerOpts {
	return func(m *CNIManager) error {
		m.DefaultSubnet = s
		return nil
	}
}

func WithDefaultIfPrefix(s string) CNIManagerOpts {
	return func(m *CNIManager) error {
		m.DefaultIfPrefix = s
		return nil
	}
}

func WithDefaultCNIConf(s string) CNIManagerOpts {
	return func(m *CNIManager) error {
		m.DefaultCNIConf = s
		return nil
	}
}

// netNamespace generates the namespace path based on task PID.
func (c *CNIManager) netNamespace(pid uint32) string {
	return fmt.Sprintf(c.NetNSPathFmt, pid)
}

// Attach implements Manager.
func (c *CNIManager) Attach(ctx context.Context, containerID string, containerPID uint32, opts ...gocni.NamespaceOpts) (*gocni.Result, error) {
	ctx, span := tracer.Start(ctx, "networking.cni.Attach")
	defer span.End()

	netnsPath := c.netNamespace(containerPID)

	return c.cni.Setup(
		ctx,
		containerID,
		netnsPath,
		opts...,
	)
}

// Check implements Manager.
func (c *CNIManager) Check(ctx context.Context, containerID string, containerPID uint32, opts ...gocni.NamespaceOpts) error {
	ctx, span := tracer.Start(ctx, "networking.cni.Check")
	defer span.End()

	netnsPath := c.netNamespace(containerPID)

	return c.cni.Check(
		ctx,
		containerID,
		netnsPath,
		opts...,
	)
}

// Detach implements Manager.
func (c *CNIManager) Detach(ctx context.Context, containerID string, containerPID uint32, opts ...gocni.NamespaceOpts) error {
	ctx, span := tracer.Start(ctx, "networking.cni.Detach")
	defer span.End()

	netnsPath := c.netNamespace(containerPID)
	return c.cni.Remove(ctx, containerID, netnsPath, opts...)
}

func NewCNIManager(opts ...CNIManagerOpts) (Manager, error) {
	m := &CNIManager{
		CNIBinDir:              "/opt/cni/bin",
		CNIConfDir:             "/etc/cni/net.d",
		NetNSPathFmt:           "/proc/%d/ns/net",
		CNIDataDir:             "/var/lib/cni/",
		DefaultCNIConfFilename: "10-voiyd.conflist",
		DefaultNetworkName:     "bridge",
		DefaultBridgeName:      "voiyd0",
		DefaultSubnet:          "10.69.0.0/16",
		DefaultSubnetGw:        "10.69.0.1",
		DefaultIfPrefix:        "eth",
	}

	m.DefaultCNIConf = fmt.Sprintf(`
{
  "cniVersion": "1.0.0",
  "name": "%s",
  "plugins": [
    {
      "type": "bridge",
      "bridge": "%s",
      "isGateway": true,
      "ipMasq": true,
      "hairpinMode": true,
      "ipam": {
        "dataDir": "%s",
        "ranges": [
          [
            {
              "subnet": "%s",
              "gateway": "%s"
            }
          ]
        ],
        "routes": [
          {
            "dst": "0.0.0.0/0"
          }
        ],
        "type": "host-local"
      }
    },
    {
      "type": "portmap",
      "capabilities": {
        "portMappings": true
      }
    },
    {
      "type": "firewall",
      "ingressPolicy": "same-bridge"
    },
    {
      "type": "tuning"
    }
  ]
}
`, m.DefaultNetworkName, m.DefaultBridgeName, m.CNIDataDir, m.DefaultSubnet, m.DefaultSubnetGw)

	// Apply manager opts
	for _, opt := range opts {
		if err := opt(m); err != nil {
			return nil, err
		}
	}

	// Append cni opts to a list of default opts
	newOpts := []gocni.Opt{
		gocni.WithMinNetworkCount(2),
		gocni.WithPluginConfDir(m.CNIConfDir),
		gocni.WithPluginDir([]string{m.CNIBinDir}),
		gocni.WithInterfacePrefix(m.DefaultIfPrefix),
	}
	newOpts = append(newOpts, m.cniOpts...)

	// Create directories
	_, err := os.Stat(m.CNIConfDir)
	if !os.IsNotExist(err) {
		if err := os.MkdirAll(m.CNIConfDir, 0o755); err != nil {
			return nil, fmt.Errorf("cannot create directory: '%s'. error: %w", m.CNIConfDir, err)
		}
	}

	netconfig := path.Join(m.CNIConfDir, m.DefaultCNIConfFilename)
	if err := os.WriteFile(netconfig, []byte(m.DefaultCNIConf), 0o644); err != nil {
		return nil, fmt.Errorf("cannot write network config: '%s'. error: %w", m.DefaultCNIConfFilename, err)
	}

	// Initialize CNI library
	cni, err := gocni.New(
		newOpts...,
	)
	if err != nil {
		return nil, fmt.Errorf("error initializing cni: %w", err)
	}

	// Append cni opts to list of defaults
	loadOpts := []gocni.Opt{
		gocni.WithLoNetwork,
		gocni.WithConfListBytes([]byte(m.DefaultCNIConf)),
	}

	// Load the cni configuration
	err = cni.Load(loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load cni configuration: %w", err)
	}

	m.cni = cni

	return m, nil
}
