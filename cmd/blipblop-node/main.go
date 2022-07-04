package main

import (
	"context"
	"fmt"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/amimof/blipblop/pkg/middleware"
	"github.com/amimof/blipblop/pkg/networking"
	"github.com/amimof/blipblop/pkg/server"
	"github.com/containerd/containerd"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/pflag"
	"log"
	"os"
	"os/signal"
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

	nodeName string

	insecureSkipVerify bool

	tlsHost           string
	tlsPort           int
	tlsCertificate    string
	tlsCertificateKey string
	tlsCACertificate  string

	metricsHost string
	metricsPort int

	containerdSocket string
)

func init() {
	pflag.StringVar(&nodeName, "node-name", "", "a name that uniquely identifies this node in the cluster")
	pflag.StringVar(&tlsHost, "tls-host", "localhost", "The host address on which to listen for the --tls-port port")
	pflag.StringVar(&tlsCertificate, "tls-certificate", "", "the certificate to use for secure connections")
	pflag.StringVar(&tlsCertificateKey, "tls-key", "", "the private key to use for secure conections")
	pflag.StringVar(&tlsCACertificate, "tls-ca", "", "the certificate authority file to be used with mutual tls auth")
	pflag.StringVar(&metricsHost, "metrics-host", "localhost", "The host address on which to listen for the --metrics-port port")
	pflag.StringVar(&containerdSocket, "containerd-socket", "/run/containerd/containerd.sock", "Path to containerd socket")
	pflag.IntVar(&tlsPort, "tls-port", 5700, "the port to listen on for secure connections, defaults to 8443")
	pflag.IntVar(&metricsPort, "metrics-port", 8889, "the port to listen on for Prometheus metrics, defaults to 8888")
	pflag.BoolVar(&insecureSkipVerify, "insecure-skip-verify", false, "whether the client should verify the server's certificate chain and host name")
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

	// node-name is required
	if nodeName == "" {
		fmt.Fprintln(os.Stderr, "node name is required")
		return
	}

	// Setup node service client
	client, err := client.New(fmt.Sprintf("%s:%d", tlsHost, tlsPort))
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s", err.Error())
	}
	defer client.Close()

	// Join node
	ctx := context.Background()
	err = client.JoinNode(ctx, nodeName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s", err.Error())
	}

	// Setup signal handler
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt)
	go func() {
		select {
		case <-done:
			err := client.ForgetNode(ctx)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s", err.Error())
			}
			log.Printf("Unjoined %s", nodeName)
		}
	}()

	// Create  containerd client
	cclient, err := containerd.New(containerdSocket)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s", err.Error())
	}
	defer cclient.Close()

	// Create networking
	cni, err := networking.InitNetwork()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s", err.Error())
	}

	// Setup controllers
	mdlwr := middleware.NewManager(
		//middleware.WithRuntime(client, cclient, cni),
		middleware.WithEvents(client, cclient, cni),
	)
	mdlwr.Run(ctx)
	defer mdlwr.Stop()

	// Metrics server
	ms := server.NewServer()
	ms.Port = metricsPort
	ms.Host = metricsHost
	ms.Name = "metrics"
	ms.Handler = promhttp.Handler()

	// Listen and serve!
	err = ms.Serve()
	if err != nil {
		log.Fatal(err)
	}

}
