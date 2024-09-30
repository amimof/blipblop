package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/amimof/blipblop/api/services/nodes/v1"
	"github.com/amimof/blipblop/api/types/v1"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/amimof/blipblop/pkg/controller"
	"github.com/amimof/blipblop/pkg/networking"
	rt "github.com/amimof/blipblop/pkg/runtime"

	//"github.com/amimof/blipblop/pkg/server"

	"github.com/containerd/containerd"
	//"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/spf13/pflag"
)

var (
	// VERSION of the app. Is set when project is built and should never be set manually
	VERSION string
	// COMMIT is the Git commit currently used when compiling. Is set when project is built and should never be set manually
	COMMIT string
	// BRANCH is the Git branch currently used when compiling. Is set when project is built and should never be set manually
	BRANCH string
	// GOVERSION used to compile. Is set when project is built and should never be set manually
	GOVERSION string

	insecureSkipVerify bool

	host              string
	port              int
	tlsCertificate    string
	tlsCertificateKey string
	tlsCACertificate  string

	metricsHost string
	metricsPort int

	containerdSocket string

	logLevel string
)

func init() {
	pflag.StringVar(&host, "host", "localhost", "The host address to connect to, defaults to localhost")
	pflag.StringVar(&tlsCertificate, "tls-certificate", "", "the certificate to use for secure connections")
	pflag.StringVar(&tlsCertificateKey, "tls-key", "", "the private key to use for secure conections")
	pflag.StringVar(&tlsCACertificate, "tls-ca", "", "the certificate authority file to be used with mutual tls auth")
	pflag.StringVar(&metricsHost, "metrics-host", "localhost", "The host address on which to listen for the --metrics-port port")
	pflag.StringVar(&containerdSocket, "containerd-socket", "/run/containerd/containerd.sock", "Path to containerd socket")
	pflag.StringVar(&logLevel, "log-level", "info", "The level of verbosity of log output")
	pflag.IntVar(&port, "port", 5700, "the port to connect to, defaults to 5700")
	pflag.IntVar(&metricsPort, "metrics-port", 8889, "the port to listen on for Prometheus metrics, defaults to 8888")
	pflag.BoolVar(&insecureSkipVerify, "insecure-skip-verify", false, "whether the client should verify the server's certificate chain and host name")
}

func parseSlogLevel(lvl string) (slog.Level, error) {
	switch strings.ToLower(lvl) {
	case "error":
		return slog.LevelError, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	}

	var l slog.Level
	return l, fmt.Errorf("not a valid log level %q", lvl)
}

func main() {
	showver := pflag.Bool("version", false, "Print version")

	pflag.Usage = func() {
		fmt.Fprint(os.Stderr, "Usage:\n")
		fmt.Fprint(os.Stderr, "  blipblop-node [OPTIONS]\n\n")

		title := "Distributed containerd"
		fmt.Fprint(os.Stderr, title+"\n\n")
		desc := "Manages multiple Kubernetes clusters and provides a single API to clients"
		if desc != "" {
			fmt.Fprintf(os.Stderr, desc+"\n\n")
		}
		fmt.Fprintln(os.Stderr, pflag.CommandLine.FlagUsages())
	}

	// parse the CLI flags
	pflag.Parse()

	// Show version if requested
	if *showver {
		fmt.Printf("Version: %s\nCommit: %s\nBranch: %s\nGoVersion: %s\n", VERSION, COMMIT, BRANCH, GOVERSION)
		return
	}

	// Setup logging
	lvl, err := parseSlogLevel(logLevel)
	if err != nil {
		fmt.Printf("error parsing log level: %v", err)
		os.Exit(1)
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))

	// Setup TLS for the client
	tlsConfig := &tls.Config{InsecureSkipVerify: insecureSkipVerify}
	if tlsCACertificate != "" {
		caCert, err := os.ReadFile(tlsCACertificate)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading CA certificate file: %v", err)
			return
		}
		certPool := x509.NewCertPool()
		if !certPool.AppendCertsFromPEM(caCert) {
			fmt.Fprintf(os.Stderr, "error appending CA certitifacte to pool: %v", err)
		}
		tlsConfig.RootCAs = certPool
	}

	// Setup mTLS
	if tlsCertificate != "" && tlsCertificateKey != "" {
		cert, err := tls.LoadX509KeyPair(tlsCertificate, tlsCertificateKey)
		if err != nil {
			log.Error("error loading x509 cert key pair", "error", err)
			os.Exit(1)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup a clientset for this node
	serverAddr := fmt.Sprintf("%s:%d", host, port)
	cs, err := client.New(ctx, serverAddr, client.WithTLSConfig(tlsConfig))
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s", err.Error())
	}
	defer cs.Close()

	// Join node
	nodeName, err := os.Hostname()
	if err != nil {
		log.Error("error getting hostname from env", "error", err)
		return
	}

	n, err := newNodeFromEnv()
	if err != nil {
		log.Error("error creating a node from environment", "error", err)
		return
	}

	err = cs.NodeV1().Join(ctx, n)
	if err != nil {
		log.Error("error joining node to server", "error", err)
		return
	}

	// Create containerd client
	cclient, err := reconnectWithBackoff(containerdSocket, log)
	if err != nil {
		log.Error("could not establish connection to containerd: %v", "error", err)
		return
	}
	defer cclient.Close()

	// Create networking
	cni, err := networking.InitNetwork()
	if err != nil {
		log.Error("error initializing networking", "error", err)
		return
	}

	// Setup and run controllers
	runtime := rt.NewContainerdRuntimeClient(cclient, cni, rt.WithLogger(log))
	exit := make(chan os.Signal, 1)
	stopCh := make(chan struct{})

	containerdCtrl := controller.NewContainerdController(cclient, cs, runtime, controller.WithContainerdControllerLogger(log))
	go containerdCtrl.Run(ctx, stopCh)
	log.Info("Started Containerd Controller")

	containerCtrl := controller.NewContainerController(cs, runtime, controller.WithContainerControllerLogger(log))
	go containerCtrl.Run(ctx, stopCh)
	log.Info("Started Container Controller")

	nodeCtrl := controller.NewNodeController(cs, runtime, controller.WithNodeControllerLogger(log))
	go nodeCtrl.Run(ctx, stopCh)
	log.Info("Started Node Controller")

	// Setup signal handler
	signal.Notify(exit, os.Interrupt, syscall.SIGTERM)
	<-exit

	stopCh <- struct{}{}

	log.Info("Shutting down")
	ctx.Done()
	if err := cs.NodeV1().Forget(ctx, nodeName); err != nil {
		fmt.Fprintf(os.Stderr, "%s", err.Error())
	}
	log.Info("Successfully unjoined from cluster", "node", nodeName)
}

func connectContainerd(address string) (*containerd.Client, error) {
	client, err := containerd.New(address)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to containerd: %w", err)
	}
	return client, nil
}

func reconnectWithBackoff(address string, l *slog.Logger) (*containerd.Client, error) {
	var (
		client *containerd.Client
		err    error
	)

	// TODO: Parameterize the backoff time
	backoff := 2 * time.Second
	for {
		client, err = connectContainerd(address)
		if err == nil {
			return client, nil
		}

		l.Error("reconnection to containerd failed", "error", err, "retry_in", backoff)
		time.Sleep(backoff)

	}
}

// NewNodeFromEnv creates a new node from the current environment with the name s
func newNodeFromEnv() (*nodes.Node, error) {
	arch := runtime.GOARCH
	oper := runtime.GOOS
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	n := &nodes.Node{
		Meta: &types.Meta{
			Name: hostname,
		},
		Status: &nodes.Status{
			Ips:      getIpAddressesAsString(),
			Hostname: hostname,
			Arch:     arch,
			Os:       oper,
			Ready:    false,
		},
	}
	return n, err
}

func getIpAddressesAsString() []string {
	var i []string
	inters, err := net.Interfaces()
	if err != nil {
		return i
	}
	for _, inter := range inters {
		addrs, err := inter.Addrs()
		if err != nil {
			return i
		}
		for _, addr := range addrs {
			a := addr.String()
			i = append(i, a)
		}
	}
	return i
}
