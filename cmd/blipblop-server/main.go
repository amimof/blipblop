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
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/amimof/blipblop/pkg/client"
	"github.com/amimof/blipblop/pkg/controller"
	"github.com/amimof/blipblop/pkg/events"
	"github.com/amimof/blipblop/pkg/instrumentation"
	"github.com/amimof/blipblop/pkg/repository"
	"github.com/amimof/blipblop/pkg/scheduling"
	"github.com/amimof/blipblop/pkg/server"
	"github.com/amimof/blipblop/services/container"
	"github.com/amimof/blipblop/services/containerset"
	"github.com/amimof/blipblop/services/event"
	logsvc "github.com/amimof/blipblop/services/log"
	"github.com/amimof/blipblop/services/node"
	"github.com/amimof/blipblop/services/volume"
	"github.com/dgraph-io/badger/v4"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
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

	tcpHost           string
	tcpPort           int
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
	otelEndpoint      string

	log *slog.Logger
)

func init() {
	pflag.StringVar(&socketPath, "socket-path", "/var/run/blipblop/blipblop.sock", "the unix socket to listen on")
	pflag.StringVar(&host, "host", "localhost", "The host address on which to listen for the --port port")
	pflag.StringVar(&tcpHost, "tcp-host", "localhost", "The host address on which to listen for the --tcp-port port")
	pflag.StringVar(&tlsHost, "tls-host", "localhost", "The host address on which to listen for the --tls-port port")
	pflag.StringVar(&tcptlsHost, "tcp-tls-host", "localhost", "The host address on which to listen for the --tcp-tls-port port")
	pflag.StringVar(&tlsCertificate, "tls-certificate", "", "the certificate to use for secure connections")
	pflag.StringVar(&tlsCertificateKey, "tls-key", "", "the private key to use for secure conections")
	pflag.StringVar(&tlsCACertificate, "tls-ca", "", "the certificate authority file to be used with mutual tls auth")
	pflag.StringVar(&metricsHost, "metrics-host", "localhost", "The host address on which to listen for the --metrics-port port")
	pflag.StringVar(&logLevel, "log-level", "info", "The level of verbosity of log output")
	pflag.StringVar(&otelEndpoint, "otel-endpoint", "", "Endpoint address of OpenTelemetry collector")
	pflag.StringSliceVar(&enabledListeners, "scheme", []string{"https", "grpc"}, "the listeners to enable, this can be repeated and defaults to the schemes in the swagger spec")

	pflag.IntVar(&port, "port", 8080, "the port to listen on for insecure connections, defaults to 8080")
	pflag.IntVar(&tcpPort, "tcp-port", 5700, "the port to listen on for insecure connections, defaults to 8080")
	pflag.IntVar(&tlsPort, "tls-port", 8443, "the port to listen on for secure connections, defaults to 8443")
	pflag.IntVar(&tcptlsPort, "tcp-tls-port", 5743, "the port to listen on for GRPC connections, defaults to 5743")
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
			fmt.Fprintf(os.Stderr, "%s\n\n", desc)
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
	// log = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
	log = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: lvl, AddSource: true}))

	// Setup TLS configuration based on flags
	var serverOpts []server.NewServerOption
	var gatewayOpts []server.NewGatewayOption
	if tlsCertificate != "" && tlsCertificateKey != "" {

		creds, err := credentials.NewServerTLSFromFile(tlsCertificate, tlsCertificateKey)
		if err != nil {
			log.Error("error loading tls certificate pair", "error", err)
			os.Exit(1)
		}
		serverOpts = append(serverOpts, server.WithGrpcOption(grpc.Creds(creds), grpc.StatsHandler(otelgrpc.NewServerHandler())))

		cert, err := tls.LoadX509KeyPair(tlsCertificate, tlsCertificateKey)
		if err != nil {
			log.Error("error loading x509 cert key pair", "error", err)
			os.Exit(1)
		}
		// TODO: This effectively forces user to provide CA certificate. Need to implement
		// so that ca cert pool is added to the grpc dial option contitionally
		caCert, err := os.ReadFile(tlsCACertificate)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading CA certificate file: %v", err)
			return
		}
		certPool := x509.NewCertPool()
		if !certPool.AppendCertsFromPEM(caCert) {
			fmt.Fprintf(os.Stderr, "error appending CA certitifacte to pool: %v", err)
		}
		gatewayOpts = append(gatewayOpts,
			server.WithGrpcDialOption(
				grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{RootCAs: certPool, Certificates: []tls.Certificate{cert}})),
			),
			server.WithTLSConfig(&tls.Config{
				Certificates: []tls.Certificate{cert},
			}),
		)
	}

	// Setup signal handlers
	exit := make(chan os.Signal, 1)
	signal.Notify(exit, os.Interrupt, syscall.SIGTERM)

	// Setup badgerdb and repo
	db, err := badger.Open(badger.DefaultOptions("/var/lib/blipblop"))
	if err != nil {
		log.Error("error opening badger database", "error", err.Error())
		os.Exit(1)
	}

	defer func() {
		if err := db.Close(); err != nil {
			log.Error("error closing database", "error", err)
		}
	}()

	// Setup event exchange bus
	exchange := events.NewExchange(events.WithExchangeLogger(log))

	// Setup services
	eventService := event.NewService(
		repository.NewEventBadgerRepository(db, repository.WithEventBadgerRepositoryMaxItems(5)),
		event.WithLogger(log),
		event.WithExchange(exchange),
	)

	nodeService := node.NewService(
		repository.NewNodeBadgerRepository(db),
		node.WithLogger(log),
		node.WithExchange(exchange),
	)

	containerSetService := containerset.NewService(
		repository.NewContainerSetBadgerRepository(db),
		containerset.WithLogger(log),
		containerset.WithExchange(exchange),
	)

	containerService := container.NewService(
		repository.NewContainerBadgerRepository(db),
		container.WithLogger(log),
		container.WithExchange(exchange),
	)

	logService := logsvc.NewService(
		logsvc.WithLogger(log),
		logsvc.WithExchange(exchange),
	)

	volumeService := volume.NewService(
		repository.NewVolumeInMemRepo(),
		volume.WithLogger(log),
		volume.WithExchange(exchange),
	)

	// Context
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Setup Metrics
	metricsOpts, err := instrumentation.InitServerMetrics(ctx)
	if err != nil {
		log.Error("Failed to start prometheus exporter", "error", err)
	}
	serverOpts = append(serverOpts, server.WithGrpcOption(metricsOpts))

	go serveMetrics(promhttp.Handler())

	// Setup server
	s, err := server.New(serverOpts...)
	if err != nil {
		log.Error("error setting up gRPC server", "error", err)
		os.Exit(1)
	}

	// Register services to gRPC server
	err = s.RegisterService(eventService, nodeService, containerSetService, containerService, logService, volumeService)
	if err != nil {
		log.Error("error registering services to server", "error", err)
		os.Exit(1)
	}
	go serveTCP(s)
	go serveUnix(s)

	// Used by clientset and the gateway to connect internally
	socketAddr := fmt.Sprintf("unix://%s", socketPath)

	// Setup tracing
	if len(otelEndpoint) > 0 {
		shutdownTraceProvider, err := instrumentation.InitTracing(ctx, "blipblop-server", VERSION, otelEndpoint)
		if err != nil {
			log.Error("error setting up tracing", "error", err)
			os.Exit(1)
		}
		defer func() {
			if err := shutdownTraceProvider(ctx); err != nil {
				log.Error("error sutting down trace provider", "error", err)
			}
		}()
	}

	// Setup gateway
	gw, err := server.NewGateway(
		ctx,
		socketAddr,
		server.DefaultMux,
		gatewayOpts...,
	)
	if err != nil {
		log.Error("error setting up gateway", "error", err)
		os.Exit(1)
	}

	go serveGateway(gw)

	// Setup a clientset for the controllers
	cs, err := client.New(socketAddr, client.WithLogger(log), client.WithTLSConfig(&tls.Config{InsecureSkipVerify: true}))
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s", err.Error())
	}
	defer func() {
		if err := cs.Close(); err != nil {
			log.Error("error closing clientset connection", "error", err)
		}
	}()

	// Start controllers
	containerSetCtrl := controller.NewContainerSetController(cs, controller.WithContainerSetLogger(log))
	go containerSetCtrl.Run(ctx)
	log.Info("Started ContainerSet Controller")

	// Start scheduler
	sched := scheduling.NewHorizontalScheduler(cs)
	schedulerCtrl := controller.NewSchedulerController(cs, sched)
	go schedulerCtrl.Run(ctx)
	log.Info("Started Scheduler Controller")

	// Wait for exit signal, begin shutdown process after this point
	<-exit
	cancel()

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

func serveMetrics(h http.Handler) {
	addr := net.JoinHostPort(metricsHost, strconv.Itoa(metricsPort))
	log.Info("metrics listening", "address", addr)
	if err := http.ListenAndServe(addr, h); err != nil {
		log.Error("error serving metrics", "error", err)
		return
	}
}

func serveGateway(gw *server.Gateway) {
	if tlsCertificate != "" && tlsCertificateKey != "" {
		addr := net.JoinHostPort(tlsHost, strconv.Itoa(tlsPort))
		l, err := net.Listen("tcp", addr)
		if err != nil {
			log.Error("error creating gateway listener", "error", err)
		}
		log.Info("gateway listening securely", "address", addr)
		if err := gw.ServeTLS(l, tlsCertificate, tlsCertificateKey); err != nil {
			log.Error("error serving gateway", "error", err)
			os.Exit(1)
		}
	}

	addr := net.JoinHostPort(host, strconv.Itoa(port))
	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Error("error creating gateway listener", "error", err)
	}
	log.Info("gateway listening", "address", addr)
	if err := gw.Serve(l); err != nil {
		log.Error("error serving gateway", "error", err)
		os.Exit(1)
	}
}

func serveUnix(s *server.Server) {
	// Remove the socket file if it already exists
	if _, err := os.Stat(socketPath); err == nil {
		if err := os.RemoveAll(socketPath); err != nil {
			log.Error("failed to remove existing Unix socket", "error", err)
			return
		}
	}

	// Create socket dir if doesn't exist
	dirPath := filepath.Dir(socketPath)
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		if err := os.MkdirAll(dirPath, 0); err != nil {
			log.Error("failed to create socket directory", "error", err)
			return
		}
	}
	unixListener, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Error("error setting up Unix socket listener", "error", err.Error())
		os.Exit(1)
	}

	log.Info("server listening", "socket", socketPath)
	if err := s.Serve(unixListener); err != nil {
		log.Error("error serving server", "error", err)
		os.Exit(1)
	}
}

func serveTCP(s *server.Server) {
	addr := net.JoinHostPort(tcpHost, strconv.Itoa(tcpPort))
	if tlsCertificate != "" && tlsCertificateKey != "" {
		addr = net.JoinHostPort(tcptlsHost, strconv.Itoa(tcptlsPort))
	}

	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Error("error setting up server listener", "error", err.Error())
		os.Exit(1)
	}
	log.Info("server listening", "address", addr)
	if err := s.Serve(l); err != nil {
		log.Error("error serving server", "error", err)
		os.Exit(1)
	}
}
