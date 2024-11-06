package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/amimof/blipblop/pkg/client"
	"github.com/amimof/blipblop/pkg/controller"
	"github.com/amimof/blipblop/pkg/networking"
	"github.com/amimof/blipblop/pkg/node"
	rt "github.com/amimof/blipblop/pkg/runtime"
	"github.com/containerd/containerd"
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
	host               string
	port               int
	tlsCertificate     string
	tlsCertificateKey  string
	tlsCACertificate   string
	metricsHost        string
	metricsPort        int
	containerdSocket   string
	logLevel           string
	nodeFile           string
)

func init() {
	pflag.StringVar(&host, "host", "localhost", "The host address to connect to, defaults to localhost")
	pflag.StringVar(&tlsCertificate, "tls-certificate", "", "the certificate to use for secure connections")
	pflag.StringVar(&tlsCertificateKey, "tls-key", "", "the private key to use for secure conections")
	pflag.StringVar(&tlsCACertificate, "tls-ca", "", "the certificate authority file to be used with mutual tls auth")
	pflag.StringVar(&metricsHost, "metrics-host", "localhost", "The host address on which to listen for the --metrics-port port")
	pflag.StringVar(&containerdSocket, "containerd-socket", "/run/containerd/containerd.sock", "Path to containerd socket")
	pflag.StringVar(&logLevel, "log-level", "info", "The level of verbosity of log output")
	pflag.StringVar(&nodeFile, "node-file", "/etc/blipblop/node.yaml", "Path to node identity file")
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
	cs, err := client.New(ctx, serverAddr, client.WithTLSConfig(tlsConfig), client.WithLogger(log))
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s", err.Error())
	}
	defer cs.Close()

	// Create containerd client
	cclient, err := reconnectWithBackoff(containerdSocket, log)
	if err != nil {
		log.Error("could not establish connection to containerd: %v", "error", err)
		return
	}
	defer cclient.Close()

	// Join node
	n, err := node.LoadNodeFromEnv(nodeFile)
	if err != nil {
		log.Error("error creating a node from environment", "error", err)
		return
	}

	err = cs.NodeV1().Join(ctx, n)
	if err != nil {
		log.Error("error joining node to server", "error", err)
		return
	}
	// Create networking
	cni, err := networking.InitNetwork()
	if err != nil {
		log.Error("error initializing networking", "error", err)
		return
	}

	// Setup and run controllers
	runtime := rt.NewContainerdRuntimeClient(cclient, cni, rt.WithLogger(log))
	exit := make(chan os.Signal, 1)

	containerdCtrl := controller.NewContainerdController(cclient, cs, runtime, controller.WithContainerdControllerLogger(log))
	go containerdCtrl.Run(ctx)
	log.Info("started containerd controller")

	nodeCtrl := controller.NewNodeController(cs, runtime, controller.WithNodeControllerLogger(log), controller.WithNodeName(n.GetMeta().GetName()))
	go nodeCtrl.Run(ctx)
	log.Info("started node controller")

	// Setup signal handler
	signal.Notify(exit, os.Interrupt, syscall.SIGTERM)
	<-exit
	cancel()

	// stopCh <- struct{}{}

	log.Info("shutting down")
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
