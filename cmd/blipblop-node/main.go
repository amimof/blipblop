package main

import (
	"context"
	"fmt"
	"github.com/amimof/blipblop/pkg/controller"
	"github.com/amimof/blipblop/pkg/event"
	"github.com/amimof/blipblop/pkg/networking"
	"github.com/amimof/blipblop/pkg/server"
	proto "github.com/amimof/blipblop/proto"
	"github.com/containerd/containerd"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/pflag"
	"google.golang.org/grpc"
	"io"
	"log"
	"os"
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
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithBlock(), grpc.WithInsecure())
	conn, err := grpc.Dial(fmt.Sprintf("%s:%d", tlsHost, tlsPort), opts...)
	if err != nil {
		log.Fatalf("Failed to dial %s with error: %s", tlsHost, err.Error())
	}
	defer conn.Close()

	ctx := context.Background()
	client := proto.NewNodeServiceClient(conn)
	go joinNode(ctx, client, nodeName)

	// Create containerd client
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
	cm := controller.NewControllerManager(controller.NewUnitController(cclient, cni))
	cm.SpawnAll()

	event.On("container-create", event.ListenerFunc(func(e *event.Event) error {
		log.Printf("Container created!", "")
		return nil
	}))

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

type nodeServiceServer struct {
	proto.UnimplementedNodeServiceServer
	channel map[string][]chan *proto.Event
}

func joinNode(ctx context.Context, client proto.NodeServiceClient, name string) {
	node := proto.Node{Id: name}
	stream, err := client.JoinNode(ctx, &node)
	if err != nil {
		log.Fatalf("join node error %s", err.Error())
	}

	log.Printf("Node %s joined", node.Id)

	waitc := make(chan struct{})
	go func() {
		for {
			in, err := stream.Recv()
			if err == io.EOF {
				log.Println("Got EOF")
				close(waitc)
				return
			}
			if err != nil {
				//log.Printf("Failed to receive events from joined node: %s", err.Error())
				continue
			}
			log.Printf("EVENT: (%v) -> %v \n", in.Node, in.Name)
		}
	}()
	<-waitc
}

func sendEvent(ctx context.Context, client proto.NodeServiceClient, event, name string) {
	stream, err := client.FireEvent(ctx)
	if err != nil {
		log.Printf("Cannot fire event: %s", err.Error())
	}

	ev := proto.Event{
		Node: &proto.Node{
			Id: name,
		},
		Name: event,
	}

	stream.Send(&ev)

	ack, err := stream.CloseAndRecv()
	if err != nil {
		log.Fatalf("Cannot send event: %s", err.Error())
	}
	log.Printf("Event sent: %v", ack)
}
