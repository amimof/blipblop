package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/amimof/blipblop/pkg/client"
	"github.com/amimof/blipblop/pkg/controller"
	"github.com/amimof/blipblop/pkg/instrumentation"
	"github.com/amimof/blipblop/pkg/networking"
	"github.com/amimof/blipblop/pkg/node"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"

	// "github.com/amimof/blipblop/pkg/runtime"
	rt "github.com/amimof/blipblop/pkg/runtime"
	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/spf13/pflag"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
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
	runtimeNamespace   string
	otelEndpoint       string
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
	pflag.StringVar(&runtimeNamespace, "namespace", rt.DefaultNamespace, "Runtime namespace to use for containers")
	pflag.StringVar(&otelEndpoint, "otel-endpoint", "", "Endpoint address of OpenTelemetry collector")
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
			fmt.Fprintf(os.Stderr, "%s\n\n", desc)
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
	ctx = namespaces.WithNamespace(ctx, runtimeNamespace)
	defer cancel()

	if len(otelEndpoint) > 0 {
		shutdownTraceProvider, err := instrumentation.InitTracing(ctx, "blipblop-node", VERSION, otelEndpoint)
		if err != nil {
			log.Error("error setting up tracing", "error", err)
			os.Exit(0)
		}
		defer func() {
			if err := shutdownTraceProvider(ctx); err != nil {
				log.Error("error shutting down trace provider", "error", err)
			}
		}()
	}

	// Setup metrics
	metricsOpts, err := instrumentation.InitClientMetrics()
	if err != nil {
		log.Error("Failed to start prometheus exporter", "error", err)
	}

	go serveMetrics(promhttp.Handler(), log)

	// Setup a clientset for this node
	serverAddr := fmt.Sprintf("%s:%d", host, port)
	cs, err := client.New(
		serverAddr,
		client.WithGrpcDialOption(
			metricsOpts,
			grpc.WithStatsHandler(
				otelgrpc.NewClientHandler(),
			),
		),
		client.WithTLSConfig(tlsConfig),
		client.WithLogger(log),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s", err.Error())
	}
	defer func() {
		if err := cs.Close(); err != nil {
			log.Error("error closing clientset connection", "error", err)
		}
	}()

	// Create containerd client
	cclient, err := reconnectWithBackoff(containerdSocket, log)
	if err != nil {
		log.Error("could not establish connection to containerd: %v", "error", err)
		return
	}
	defer func() {
		if err := cclient.Close(); err != nil {
			log.Error("error closing containerd connection", "error", err)
		}
	}()

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
	cni, err := networking.NewCNIManager()
	if err != nil {
		log.Error("error initializing networking", "error", err)
		return
	}

	// Setup and run controllers
	runtime := rt.NewContainerdRuntimeClient(cclient, cni, rt.WithLogger(log), rt.WithNamespace(runtimeNamespace))
	exit := make(chan os.Signal, 1)

	containerdCtrl := controller.NewContainerdController(cclient, cs, runtime, controller.WithContainerdControllerLogger(log))
	go containerdCtrl.Run(ctx)
	log.Info("started containerd controller")

	nodeCtrl, err := controller.NewNodeController(cs, runtime, controller.WithNodeControllerLogger(log), controller.WithNodeName(n.GetMeta().GetName()))
	if err != nil {
		log.Error("error setting up Node Controller", "error", err)
		return
	}
	go nodeCtrl.Run(ctx)
	log.Info("started node controller")

	// Setup signal handler
	signal.Notify(exit, os.Interrupt, syscall.SIGTERM)
	<-exit
	cancel()

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

func serveMetrics(h http.Handler, l *slog.Logger) {
	addr := net.JoinHostPort(metricsHost, strconv.Itoa(metricsPort))
	l.Info("metrics listening", "address", addr)
	if err := http.ListenAndServe(addr, h); err != nil {
		fmt.Printf("error serving metrics", "error", err)
		return
	}
}
