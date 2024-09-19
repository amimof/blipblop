package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/amimof/blipblop/pkg/repository"
	"github.com/amimof/blipblop/pkg/server"
	"github.com/amimof/blipblop/services/container"
	"github.com/amimof/blipblop/services/event"
	"github.com/amimof/blipblop/services/node"
	"github.com/dgraph-io/badger/v4"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
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

	enabledListeners []string
	cleanupTimeout   time.Duration
	maxHeaderSize    uint64

	socketPath string

	host         string
	port         int
	metricsHost  string
	metricsPort  int
	listenLimit  int
	keepAlive    time.Duration
	readTimeout  time.Duration
	writeTimeout time.Duration
	logLevel     string

	tcptlsHost        string
	tcptlsPort        int
	tlsHost           string
	tlsPort           int
	tlsListenLimit    int
	tlsKeepAlive      time.Duration
	tlsReadTimeout    time.Duration
	tlsWriteTimeout   time.Duration
	tlsCertificate    string
	tlsCertificateKey string
	tlsCACertificate  string
)

func init() {
	pflag.StringVar(&socketPath, "socket-path", "/var/run/blipblop.sock", "the unix socket to listen on")
	pflag.StringVar(&host, "host", "localhost", "The host address on which to listen for the --port port")
	pflag.StringVar(&tlsHost, "tls-host", "localhost", "The host address on which to listen for the --tls-port port")
	pflag.StringVar(&tcptlsHost, "tcp-tls-host", "localhost", "The host address on which to listen for the --tcp-tls-port port")
	pflag.StringVar(&tlsCertificate, "tls-certificate", "", "the certificate to use for secure connections")
	pflag.StringVar(&tlsCertificateKey, "tls-key", "", "the private key to use for secure conections")
	pflag.StringVar(&tlsCACertificate, "tls-ca", "", "the certificate authority file to be used with mutual tls auth")
	pflag.StringVar(&metricsHost, "metrics-host", "localhost", "The host address on which to listen for the --metrics-port port")
	pflag.StringVar(&logLevel, "log-level", "info", "The level of verbosity of log output")
	pflag.StringSliceVar(&enabledListeners, "scheme", []string{"https", "grpc"}, "the listeners to enable, this can be repeated and defaults to the schemes in the swagger spec")

	pflag.IntVar(&port, "port", 8080, "the port to listen on for insecure connections, defaults to 8080")
	pflag.IntVar(&tlsPort, "tls-port", 8443, "the port to listen on for secure connections, defaults to 8443")
	pflag.IntVar(&tcptlsPort, "tcp-tls-port", 5700, "the port to listen on for GRPC connections, defaults to 5700")
	pflag.IntVar(&metricsPort, "metrics-port", 8888, "the port to listen on for Prometheus metrics, defaults to 8888")
	pflag.IntVar(&listenLimit, "listen-limit", 0, "limit the number of outstanding requests")
	pflag.IntVar(&tlsListenLimit, "tls-listen-limit", 0, "limit the number of outstanding requests")
	pflag.Uint64Var(&maxHeaderSize, "max-header-size", 1000000, "controls the maximum number of bytes the server will read parsing the request header's keys and values, including the request line. It does not limit the size of the request body")

	pflag.DurationVar(&cleanupTimeout, "cleanup-timeout", 10*time.Second, "grace period for which to wait before shutting down the server")
	pflag.DurationVar(&keepAlive, "keep-alive", 3*time.Minute, "sets the TCP keep-alive timeouts on accepted connections. It prunes dead TCP connections ( e.g. closing laptop mid-download)")
	pflag.DurationVar(&readTimeout, "read-timeout", 30*time.Second, "maximum duration before timing out read of the request")
	pflag.DurationVar(&writeTimeout, "write-timeout", 30*time.Second, "maximum duration before timing out write of the response")
	pflag.DurationVar(&tlsKeepAlive, "tls-keep-alive", 3*time.Minute, "sets the TCP keep-alive timeouts on accepted connections. It prunes dead TCP connections ( e.g. closing laptop mid-download)")
	pflag.DurationVar(&tlsReadTimeout, "tls-read-timeout", 30*time.Second, "maximum duration before timing out read of the request")
	pflag.DurationVar(&tlsWriteTimeout, "tls-write-timeout", 30*time.Second, "maximum duration before timing out write of the response")

	// Create build_info metrics
	if err := prometheus.Register(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "blipblop_build_info",
			Help: "A constant gauge with build info labels.",
			ConstLabels: prometheus.Labels{
				"branch":    BRANCH,
				"goversion": GOVERSION,
				"commit":    COMMIT,
				"version":   VERSION,
			},
		},
		func() float64 { return 1 },
	)); err != nil {
		logrus.Printf("Unable to register 'blipblop_build_info metric %s'", err.Error())
	}
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
	return l, fmt.Errorf("not a valid log level: %q", lvl)
}

func main() {
	showver := pflag.Bool("version", false, "Print version")

	pflag.Usage = func() {
		fmt.Fprint(os.Stderr, "Usage:\n")
		fmt.Fprint(os.Stderr, "  blipblop [OPTIONS]\n\n")

		title := "Kubernetes multi-cluster manager"
		fmt.Fprint(os.Stderr, title+"\n\n")
		desc := "Manages multiple Kubernetes clusters and provides a single API to clients"
		if desc != "" {
			fmt.Fprintf(os.Stderr, desc+"\n\n")
		}
		fmt.Fprintln(os.Stderr, pflag.CommandLine.FlagUsages())
	}

	// Parse the CLI flags
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

	// Setup signal handlers
	exit := make(chan os.Signal, 1)
	signal.Notify(exit, os.Interrupt, syscall.SIGTERM)

	// Setup badgerdb and repo
	db, err := badger.Open(badger.DefaultOptions("/var/lib/blipblop"))
	if err != nil {
		log.Error("error opening badger database", "error", err.Error())
		os.Exit(1)
	}
	defer db.Close()

	// Setup services
	eventService := event.NewService(repository.NewEventBadgerRepository(db))
	nodeService := node.NewService(repository.NewNodeBadgerRepository(db), eventService)
	containerService := container.NewService(repository.NewContainerBadgerRepository(db), eventService, container.WithLogger(log))

	// Setup server
	lis, err := net.Listen("tcp", net.JoinHostPort(tcptlsHost, strconv.Itoa(tcptlsPort)))
	if err != nil {
		log.Error("error setting up server listener", "error", err.Error())
		os.Exit(1)
	}
	srvAddr := net.JoinHostPort(tcptlsHost, strconv.Itoa(tcptlsPort))
	s := server.New(srvAddr)

	err = s.RegisterService(eventService, nodeService, containerService)
	if err != nil {
		log.Error("error registering services to server", "error", err)
		os.Exit(1)
	}
	go func() {
		log.Info("server listening", "host", tcptlsHost, "port", tcptlsPort)
		if err := s.Serve(lis); err != nil {
			log.Error("error serving server", "error", err)
			os.Exit(1)
		}
	}()

	// Setup gateway
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	gw, err := server.NewGateway(ctx, fmt.Sprintf("dns:///%s", net.JoinHostPort(tcptlsHost, strconv.Itoa(tcptlsPort))), server.DefaultMux)
	if err != nil {
		log.Error("error setting up gateway", "error", err)
		os.Exit(1)
	}
	glis, err := net.Listen("tcp", net.JoinHostPort(tlsHost, strconv.Itoa(tlsPort)))
	if err != nil {
		log.Error("error setting up listener for gateway", "error", err)
		os.Exit(1)
	}
	go func() {
		log.Info("gateway listening", "host", tlsHost, "port", tlsPort)
		if err := gw.Serve(glis); err != nil {
			log.Error("error serving gateway", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for exit signal
	<-exit

	// Shut down gateway
	if err := gw.Shutdown(ctx); err != nil {
		log.Error("error shutting down gateway", "error", err)
	}
	log.Info("shutting down gateway")

	// Shut down server
	go func() {
		time.Sleep(time.Second * 10)
		log.Info("deadline exceeded, shutting down forcefully")
		s.ForceShutdown()
	}()

	s.Shutdown()
	log.Info("shutting down server")
	close(exit)
}
