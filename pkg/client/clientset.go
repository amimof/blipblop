package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"sync"
	"time"

	containerv1 "github.com/amimof/blipblop/pkg/client/container/v1"
	eventv1 "github.com/amimof/blipblop/pkg/client/event/v1"
	nodev1 "github.com/amimof/blipblop/pkg/client/node/v1"
	"github.com/spf13/pflag"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

var DefaultTLSConfig = &tls.Config{
	InsecureSkipVerify: false,
}

type ClientSet struct {
	conn              *grpc.ClientConn
	nodeV1Client      *nodev1.ClientV1
	containerV1Client *containerv1.ClientV1
	eventV1Client     *eventv1.ClientV1
	mu                sync.Mutex
	grpcOpts          []grpc.DialOption
	tlsConfig         *tls.Config
}

type NewClientOption func(c *ClientSet) error

func WithGrpcDialOption(opts ...grpc.DialOption) NewClientOption {
	return func(c *ClientSet) error {
		c.grpcOpts = opts
		return nil
	}
}

func WithTLSConfig(t *tls.Config) NewClientOption {
	return func(c *ClientSet) error {
		c.tlsConfig = t
		return nil
	}
}

func WithTLSConfigFromFlags(f *pflag.FlagSet) NewClientOption {
	insecure, _ := f.GetBool("insecure")
	tlsCertificate, _ := f.GetString("tls-certificate")
	tlsCertificateKey, _ := f.GetString("tls-certificate-key")
	tlsCaCertificate, _ := f.GetString("tls-ca-certificate")

	tlsConfig := &tls.Config{
		InsecureSkipVerify: insecure,
	}
	return func(c *ClientSet) error {
		// Add CA cert to tls config
		if tlsCaCertificate != "" {
			caCert, err := os.ReadFile(tlsCaCertificate)
			if err != nil {
				return fmt.Errorf("error reading ca cert file: %v", err)
			}
			certPool := x509.NewCertPool()
			if !certPool.AppendCertsFromPEM(caCert) {
				return fmt.Errorf("error appending CA certitifacte to pool: %v", err)
			}
			tlsConfig.RootCAs = certPool
		}

		// Add certificate pair to tls config
		if tlsCertificate != "" && tlsCertificateKey != "" {
			cert, err := tls.LoadX509KeyPair(tlsCertificate, tlsCertificateKey)
			if err != nil {
				return fmt.Errorf("error loading x509 cert key pair: %v", err)
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		}

		c.tlsConfig = tlsConfig
		return nil
	}
}

func (c *ClientSet) NodeV1() *nodev1.ClientV1 {
	return c.nodeV1Client
}

func (c *ClientSet) ContainerV1() *containerv1.ClientV1 {
	return c.containerV1Client
}

func (c *ClientSet) EventV1() *eventv1.ClientV1 {
	return c.eventV1Client
}

func (c *ClientSet) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	// err := c.NodeV1().Forget(context.Background(), c.name)
	// if err != nil {
	// 	return err
	// }
	c.conn.Close()
	return nil
}

func New(ctx context.Context, server string, opts ...NewClientOption) (*ClientSet, error) {
	// Define connection backoff policy
	backoffConfig := backoff.Config{
		BaseDelay:  time.Second,       // Initial delay before retry
		Multiplier: 1.6,               // Multiplier for successive retries
		MaxDelay:   120 * time.Second, // Maximum delay
	}

	// Define keepalive parameters
	keepAliveParams := keepalive.ClientParameters{
		Time:                10 * time.Minute, // Ping the server if no activity
		Timeout:             20 * time.Second, // Timeout for server response
		PermitWithoutStream: true,             // Ping even without active streams
	}

	// Default options
	defaultOpts := []grpc.DialOption{
		grpc.WithKeepaliveParams(keepAliveParams),
		grpc.WithConnectParams(
			grpc.ConnectParams{
				Backoff:           backoffConfig,
				MinConnectTimeout: 20 * time.Second,
			},
		),
	}

	// Default clientset
	c := &ClientSet{
		grpcOpts: defaultOpts,
		tlsConfig: &tls.Config{
			InsecureSkipVerify: false,
		},
	}

	// Allow passing in custom dial options
	for _, opt := range opts {
		err := opt(c)
		if err != nil {
			return nil, err
		}
	}

	// We always want TLS but TLSConfig might be changed by the user so that's why do this here
	c.grpcOpts = append(c.grpcOpts, grpc.WithTransportCredentials(credentials.NewTLS(c.tlsConfig)))

	conn, err := grpc.DialContext(ctx, server, c.grpcOpts...)
	if err != nil {
		return nil, err
	}

	c.conn = conn
	c.nodeV1Client = nodev1.NewClientV1(conn)
	c.containerV1Client = containerv1.NewClientV1(conn)
	c.eventV1Client = eventv1.NewClientV1(conn)

	return c, nil
}
