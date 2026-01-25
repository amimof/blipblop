package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"buf.build/go/protovalidate"
	"github.com/dgraph-io/badger/v4"
	protovalidate_middleware "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/protovalidate"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/amimof/voiyd/pkg/client"
	conditionctrl "github.com/amimof/voiyd/pkg/controller/condition"
	containersetctrl "github.com/amimof/voiyd/pkg/controller/containerset"
	leasectrl "github.com/amimof/voiyd/pkg/controller/lease"
	schedulerctrl "github.com/amimof/voiyd/pkg/controller/scheduler"
	"github.com/amimof/voiyd/pkg/events"
	"github.com/amimof/voiyd/pkg/instrumentation"
	"github.com/amimof/voiyd/pkg/repository"
	"github.com/amimof/voiyd/pkg/scheduling"
	"github.com/amimof/voiyd/pkg/server"
	"github.com/amimof/voiyd/services/containerset"
	"github.com/amimof/voiyd/services/event"
	"github.com/amimof/voiyd/services/lease"
	logsvc "github.com/amimof/voiyd/services/log"
	"github.com/amimof/voiyd/services/node"
	"github.com/amimof/voiyd/services/task"
	"github.com/amimof/voiyd/services/volume"
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

	enabledListeners  []string
	cleanupTimeout    time.Duration
	maxHeaderSize     uint64
	socketPath        string
	serverAddress     string
	metricsAddress    string
	gatewayAddress    string
	listenLimit       int
	keepAlive         time.Duration
	readTimeout       time.Duration
	writeTimeout      time.Duration
	logLevel          string
	tlsListenLimit    int
	tlsKeepAlive      time.Duration
	tlsReadTimeout    time.Duration
	tlsWriteTimeout   time.Duration
	tlsCertificate    string
	tlsCertificateKey string
	tlsCACertificate  string
	otelEndpoint      string
	dbPath            string
	leaseTTLSeconds   uint32
	log               *slog.Logger
)

func init() {
	pflag.StringVar(&serverAddress, "server-address", "0.0.0.0:5743", "Address to listen the TCP server on")
	pflag.StringVar(&metricsAddress, "metrics-address", "0.0.0.0:8888", "Address to listen the metrics server on")
	pflag.StringVar(&gatewayAddress, "gateway-address", "0.0.0.0:8443", "Address to listen the http gateway server on")
	pflag.StringVar(&socketPath, "socket-path", "/var/run/voiyd/voiyd.sock", "the unix socket to listen on")
	pflag.StringVar(&tlsCertificate, "tls-certificate", "", "the certificate to use for secure connections")
	pflag.StringVar(&tlsCertificateKey, "tls-key", "", "the private key to use for secure conections")
	pflag.StringVar(&tlsCACertificate, "tls-ca", "", "the certificate authority file to be used with mutual tls auth")
	pflag.StringVar(&logLevel, "log-level", "info", "The level of verbosity of log output")
	pflag.StringVar(&otelEndpoint, "otel-endpoint", "", "Endpoint address of OpenTelemetry collector")
	pflag.StringVar(&dbPath, "db-path", "/var/lib/voiyd/db", "Directory to store database state")
	pflag.StringSliceVar(&enabledListeners, "scheme", []string{"https", "grpc"}, "the listeners to enable, this can be repeated and defaults to the schemes in the swagger spec")

	pflag.IntVar(&listenLimit, "listen-limit", 0, "limit the number of outstanding requests")
	pflag.IntVar(&tlsListenLimit, "tls-listen-limit", 0, "limit the number of outstanding requests")
	pflag.Uint64Var(&maxHeaderSize, "max-header-size", 1000000, "controls the maximum number of bytes the server will read parsing the request header's keys and values, including the request line. It does not limit the size of the request body")
	pflag.Uint32Var(&leaseTTLSeconds, "lease-ttl-seconds", 60, "how long a lease can live in seconds before it expires")

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
			Name: "voiyd_build_info",
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
		logrus.Printf("Unable to register 'voiyd_build_info metric %s'", err.Error())
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
		fmt.Fprint(os.Stderr, "  voiyd [OPTIONS]\n\n")

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

	// Load in certificates either from flags or auto-generated
	cert, err := generateCertificates()
	if err != nil {
		log.Error("error loading x509 certificates", "error", err)
		os.Exit(1)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	// Enable mTLS for gRPC server, if CA cert provided
	if tlsCACertificate != "" {
		caCert, err := os.ReadFile(tlsCACertificate)
		if err != nil {
			log.Error("error reading CA certificate file", "error", err)
			os.Exit(1)
		}
		certPool := x509.NewCertPool()
		if !certPool.AppendCertsFromPEM(caCert) {
			log.Error("error appending CA certificate to pool")
			os.Exit(1)
		}

		tlsConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
			ClientAuth:   tls.RequireAndVerifyClientCert, // ← mTLS
			ClientCAs:    certPool,
		}
		log.Info("mutual TLS enabled for gRPC server")
	}

	creds := credentials.NewTLS(tlsConfig)
	serverOpts = append(serverOpts,
		server.WithGrpcOption(grpc.Creds(creds),
			grpc.StatsHandler(otelgrpc.NewServerHandler()),
		),
	)

	tlsConfigGw := &tls.Config{
		InsecureSkipVerify: true,
	}
	gatewayOpts = append(gatewayOpts,
		server.WithTLSConfig(tlsConfig),
		server.WithGrpcDialOption(grpc.WithTransportCredentials(credentials.NewTLS(tlsConfigGw))),
	)

	// Setup signal handlers
	exit := make(chan os.Signal, 1)
	signal.Notify(exit, os.Interrupt, syscall.SIGTERM)

	// Setup badgerdb and repo
	db, err := badger.Open(badger.DefaultOptions(dbPath))
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
		repository.NewEventBadgerRepository(db),
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

	taskService := task.NewService(
		repository.NewTaskBadgerRepository(db),
		task.WithLogger(log),
		task.WithExchange(exchange),
	)

	logService := logsvc.NewService(
		logsvc.WithLogger(log),
		logsvc.WithExchange(exchange),
	)

	volumeService := volume.NewService(
		repository.NewVolumeBadgerRepository(db),
		volume.WithLogger(log),
		volume.WithExchange(exchange),
	)

	leaseService := lease.NewService(
		repository.NewLeaseBadgerRepository(db),
		lease.WithLogger(log),
		lease.WithExchange(exchange),
		lease.WithTTL(leaseTTLSeconds),
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

	validator, err := protovalidate.New()
	if err != nil {
		log.Error("Failed to create protovalidate validator", "error", err)
	}

	serverOpts = append(serverOpts, server.WithGrpcOption(metricsOpts), server.WithGrpcOption(grpc.UnaryInterceptor(protovalidate_middleware.UnaryServerInterceptor(validator))))

	go serveMetrics(metricsAddress, promhttp.Handler())

	// Setup server
	s, err := server.New(serverOpts...)
	if err != nil {
		log.Error("error setting up gRPC server", "error", err)
		os.Exit(1)
	}

	// Register services to gRPC server
	err = s.RegisterService(
		eventService,
		nodeService,
		containerSetService,
		taskService,
		logService,
		volumeService,
		leaseService,
	)
	if err != nil {
		log.Error("error registering services to server", "error", err)
		os.Exit(1)
	}
	go serveTCP(serverAddress, s)
	go serveUnix(s)

	// Used by clientset and the gateway to connect internally
	socketAddr := fmt.Sprintf("unix://%s", socketPath)

	// Setup tracing
	if len(otelEndpoint) > 0 {
		shutdownTraceProvider, err := instrumentation.InitTracing(ctx, "voiyd-server", VERSION, otelEndpoint)
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

	go serveGateway(gatewayAddress, gw)

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
	containerSetCtrl := containersetctrl.New(cs, containersetctrl.WithLogger(log))
	go containerSetCtrl.Run(ctx)
	log.Info("Started ContainerSet Controller")

	// Start scheduler
	sched := scheduling.NewHorizontalScheduler(cs)
	schedulerCtrl := schedulerctrl.New(cs, sched, schedulerctrl.WithLogger(log), schedulerctrl.WithExchange(exchange))
	go schedulerCtrl.Run(ctx)
	log.Info("Started Scheduler Controller")

	// Lease controller
	leaseCtrl := leasectrl.New(cs, leasectrl.WithLogger(log), leasectrl.WithExchange(exchange))
	go leaseCtrl.Run(ctx)
	log.Info("Started Lease Controller")

	// Conditions controller
	conditionCtrl := conditionctrl.New(cs, conditionctrl.WithLogger(log), conditionctrl.WithExchange(exchange))
	go conditionCtrl.Run(ctx)
	log.Info("Started Condition Controller")

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

func serveMetrics(addr string, h http.Handler) {
	log.Info("metrics listening", "address", addr)
	if err := http.ListenAndServe(addr, h); err != nil {
		log.Error("error serving metrics", "error", err)
		return
	}
}

func serveGateway(addr string, gw *server.Gateway) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Error("error creating gateway listener", "error", err)
	}
	if err := gw.ServeTLS(l, tlsCertificate, tlsCertificateKey); err != nil {
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

func serveTCP(addr string, s *server.Server) {
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

func generateCertificates() (tls.Certificate, error) {
	if tlsCertificate != "" && tlsCertificateKey != "" {
		return tls.LoadX509KeyPair(tlsCertificate, tlsCertificateKey)
	}

	cert := tls.Certificate{}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return cert, err
	}

	// Valid for 1 year
	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * time.Hour) // Valid for 1 year

	parent := &x509.Certificate{
		KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
			x509.ExtKeyUsageClientAuth,
		},
		IsCA:                  false,
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost", "voiyd-node"},
		IPAddresses: []net.IP{
			net.IPv4(127, 0, 0, 1),
		},
		Subject: pkix.Name{
			CommonName:         "voiyd-node",
			Country:            []string{"SE"},
			Province:           []string{"Halland"},
			Locality:           []string{"Varberg"},
			Organization:       []string{"voiyd-node"},
			OrganizationalUnit: []string{"voiyd"},
		},
		SerialNumber: serial,
		NotAfter:     notAfter,
		NotBefore:    notBefore,
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return cert, err
	}

	certData, err := x509.CreateCertificate(rand.Reader, parent, parent, &key.PublicKey, key)
	if err != nil {
		return cert, err
	}

	cert = tls.Certificate{
		Certificate: [][]byte{certData}, // Raw DER bytes from x509.CreateCertificate
		PrivateKey:  key,                // *ecdsa.PrivateKey directly
	}

	log.Info("generated x509 key pair")

	return cert, nil
}
