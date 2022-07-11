package main

import (
	"fmt"
	"github.com/amimof/blipblop/internal/api"
	//"github.com/amimof/blipblop/pkg/server"
	//"github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus"
	//"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/pflag"
	//"github.com/uber/jaeger-client-go"
	//"github.com/uber/jaeger-client-go/config"
	"log"
	"net"
	"os"
	"time"
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

	tlsHost           string
	tlsPort           int
	tlsListenLimit    int
	tlsKeepAlive      time.Duration
	tlsReadTimeout    time.Duration
	tlsWriteTimeout   time.Duration
	tlsCertificate    string
	tlsCertificateKey string
	tlsCACertificate  string
	containerdSocket  string
)

func init() {
	pflag.StringVar(&socketPath, "socket-path", "/var/run/blipblop.sock", "the unix socket to listen on")
	pflag.StringVar(&host, "host", "localhost", "The host address on which to listen for the --port port")
	pflag.StringVar(&tlsHost, "tls-host", "localhost", "The host address on which to listen for the --tls-port port")
	pflag.StringVar(&tlsCertificate, "tls-certificate", "", "the certificate to use for secure connections")
	pflag.StringVar(&tlsCertificateKey, "tls-key", "", "the private key to use for secure conections")
	pflag.StringVar(&tlsCACertificate, "tls-ca", "", "the certificate authority file to be used with mutual tls auth")
	pflag.StringVar(&metricsHost, "metrics-host", "localhost", "The host address on which to listen for the --metrics-port port")
	pflag.StringSliceVar(&enabledListeners, "scheme", []string{"https"}, "the listeners to enable, this can be repeated and defaults to the schemes in the swagger spec")

	pflag.IntVar(&port, "port", 8080, "the port to listen on for insecure connections, defaults to 8080")
	pflag.IntVar(&tlsPort, "tls-port", 8443, "the port to listen on for secure connections, defaults to 8443")
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
		log.Printf("Unable to register 'blipblop_build_info metric %s'", err.Error())
	}

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

	// parse the CLI flags
	pflag.Parse()

	// Show version if requested
	if *showver {
		fmt.Printf("Version: %s\nCommit: %s\nBranch: %s\nGoVersion: %s\n", VERSION, COMMIT, BRANCH, GOVERSION)
		return
	}

	// Setup listener for the grpc server
	endpoint := "0.0.0.0:5700"
	lis, err := net.Listen("tcp", endpoint)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s", err.Error())
	}

	// Setup the API and run the gRPC server
	a := api.NewAPIv1()
	go a.GrpcServer().Serve(lis)
	log.Printf("Started gRPC server at %s", endpoint)

	// Run HTTP server
	log.Printf("Running HTTP Server at %s", "0.0.0.0:8443")
	if err := a.Run(endpoint); err != nil {
		log.Fatal(err)
	}

	// // Create the server
	// s := &server.Server{
	// 	EnabledListeners:  enabledListeners,
	// 	CleanupTimeout:    cleanupTimeout,
	// 	MaxHeaderSize:     maxHeaderSize,
	// 	SocketPath:        socketPath,
	// 	Host:              host,
	// 	Port:              port,
	// 	ListenLimit:       listenLimit,
	// 	KeepAlive:         keepAlive,
	// 	ReadTimeout:       readTimeout,
	// 	WriteTimeout:      writeTimeout,
	// 	TLSHost:           tlsHost,
	// 	TLSPort:           tlsPort,
	// 	TLSCertificate:    tlsCertificate,
	// 	TLSCertificateKey: tlsCertificateKey,
	// 	TLSCACertificate:  tlsCACertificate,
	// 	TLSListenLimit:    tlsListenLimit,
	// 	TLSKeepAlive:      tlsKeepAlive,
	// 	TLSReadTimeout:    tlsReadTimeout,
	// 	TLSWriteTimeout:   tlsWriteTimeout,
	// 	Handler:           a.Handler(),
	// }

	// // Metrics server
	// ms := server.NewServer()
	// ms.Port = metricsPort
	// ms.Host = metricsHost
	// ms.Name = "metrics"
	// ms.Handler = promhttp.Handler()
	// go ms.Serve()

	// // Setup opentracing
	// cfg := config.Configuration{
	// 	ServiceName: "blipblop",
	// 	Sampler: &config.SamplerConfig{
	// 		Type:  "const",
	// 		Param: 1,
	// 	},
	// 	Reporter: &config.ReporterConfig{
	// 		LogSpans:            true,
	// 		BufferFlushInterval: 1 * time.Second,
	// 	},
	// }
	// tracer, closer, err := cfg.New("blipblop", config.Logger(jaeger.StdLogger))
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// opentracing.SetGlobalTracer(tracer)
	// defer closer.Close()

	// // Listen and serve!
	// err = s.Serve()
	// if err != nil {
	// 	log.Fatal(err)
	// }

}
