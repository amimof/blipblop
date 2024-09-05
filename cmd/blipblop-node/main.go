package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/amimof/blipblop/pkg/client"
	nodev1 "github.com/amimof/blipblop/pkg/client/node/v1"
	"github.com/amimof/blipblop/pkg/controller"
	"github.com/amimof/blipblop/pkg/networking"
	"github.com/amimof/blipblop/pkg/runtime"

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

	// Setup a clientset for this node
	// ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second) // define how long you want to wait for connection to be restored before giving up
	// defer cancel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ctx := context.Background()
	cs, err := client.New(ctx, fmt.Sprintf("%s:%d", tlsHost, tlsPort))
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s", err.Error())
	}
	defer cs.Close()

	// Join node
	err = cs.NodeV1().JoinNode(ctx, nodev1.NewNodeFromEnv(nodeName))
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s", err.Error())
	}

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

	// Setup and run controllers
	runtime := runtime.NewContainerdRuntimeClient(cclient, cni)
	exit := make(chan os.Signal, 1)
	stopCh := make(chan struct{})

	containerdCtrl := controller.NewContainerdController(cclient, cs, runtime)
	go containerdCtrl.Run(ctx, stopCh)
	log.Println("Started Containerd Controller")

	containerCtrl := controller.NewContainerController(cs, runtime)
	go containerCtrl.Run(ctx, stopCh)
	log.Println("Started Container Controller")

	nodeCtrl := controller.NewNodeController(cs, runtime)
	go nodeCtrl.Run(ctx, stopCh)
	log.Println("Started Node Controller")

	// Setup signal handler
	signal.Notify(exit, os.Interrupt, syscall.SIGTERM)
	<-exit

	stopCh <- struct{}{}

	log.Println("Shutting down")
	ctx.Done()
	if err := cs.NodeV1().ForgetNode(ctx, nodeName); err != nil {
		fmt.Fprintf(os.Stderr, "%s", err.Error())
	}
	log.Println("Successfully unjoined from cluster", nodeName)
}
