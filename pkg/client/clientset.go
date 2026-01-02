// Package client provides a client interface to interact with server APIs
package client

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"sync"
	"time"

	containerv1 "github.com/amimof/blipblop/pkg/client/container/v1"
	containersetv1 "github.com/amimof/blipblop/pkg/client/containerset/v1"
	eventv1 "github.com/amimof/blipblop/pkg/client/event/v1"
	logv1 "github.com/amimof/blipblop/pkg/client/log/v1"
	nodev1 "github.com/amimof/blipblop/pkg/client/node/v1"
	volumev1 "github.com/amimof/blipblop/pkg/client/volume/v1"
	"github.com/amimof/blipblop/pkg/logger"
	"github.com/google/uuid"
	"github.com/spf13/pflag"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

var DefaultTLSConfig = &tls.Config{
	InsecureSkipVerify: false,
}

type NewClientOption func(c *ClientSet) error

func WithClientID(id string) NewClientOption {
	return func(c *ClientSet) error {
		c.clientId = id
		return nil
	}
}

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

func WithLogger(l logger.Logger) NewClientOption {
	return func(c *ClientSet) error {
		c.logger = l
		return nil
	}
}

func WithTLSConfigFromFlags(f *pflag.FlagSet) NewClientOption {
	insecure, _ := f.GetBool("insecure")
	tlsCertificate, _ := f.GetString("tls-certificate")
	tlsCertificateKey, _ := f.GetString("tls-certificate-key")
	tlsCaCertificate, _ := f.GetString("tls-ca-certificate")
	return func(c *ClientSet) error {
		tlsConfig, err := getTLSConfig(tlsCertificate, tlsCertificateKey, tlsCaCertificate, insecure)
		if err != nil {
			return err
		}
		c.tlsConfig = tlsConfig
		return nil
	}
}

// WithTLSConfigFromCfg returns a NewClientOption using the provided client.Config.
// It runs Validate() on the config before returning
func WithTLSConfigFromCfg(cfg *Config) NewClientOption {
	return func(c *ClientSet) error {
		if err := cfg.Validate(); err != nil {
			return err
		}

		current, err := cfg.CurrentServer()
		if err != nil {
			return err
		}

		insecure := current.TLSConfig.Insecure
		tlsCertificate := current.TLSConfig.Certificate
		tlsCertificateKey := current.TLSConfig.Key
		tlsCaCertificate := current.TLSConfig.CA

		tlsConfig, err := getTLSConfig(tlsCertificate, tlsCertificateKey, tlsCaCertificate, insecure)
		if err != nil {
			return err
		}
		c.tlsConfig = tlsConfig
		return nil
	}
}

func getTLSConfig(cert, key, ca string, insecure bool) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: insecure,
	}

	if ca != "" {
		certPool := x509.NewCertPool()
		if !certPool.AppendCertsFromPEM([]byte(ca)) {
			return nil, fmt.Errorf("error appending CA certitifacte to pool")
		}
		tlsConfig.RootCAs = certPool
	}

	// Add certificate pair to tls config
	if cert != "" && key != "" {
		certificate, err := tls.X509KeyPair([]byte(cert), []byte(key))
		if err != nil {
			return nil, fmt.Errorf("error loading x509 cert key pair: %v", err)
		}
		tlsConfig.Certificates = []tls.Certificate{certificate}
	}
	return tlsConfig, nil
}

type ClientSet struct {
	conn                 *grpc.ClientConn
	NodeV1Client         nodev1.ClientV1
	ContainerV1Client    containerv1.ClientV1
	containerSetV1Client *containersetv1.ClientV1
	eventV1Client        *eventv1.ClientV1
	logV1Client          *logv1.ClientV1
	volumeV1Client       volumev1.ClientV1
	mu                   sync.Mutex
	grpcOpts             []grpc.DialOption
	tlsConfig            *tls.Config
	clientId             string
	logger               logger.Logger
}

func (c *ClientSet) NodeV1() nodev1.ClientV1 {
	return c.NodeV1Client
}

func (c *ClientSet) ContainerSetV1() *containersetv1.ClientV1 {
	return c.containerSetV1Client
}

func (c *ClientSet) ContainerV1() containerv1.ClientV1 {
	return c.ContainerV1Client
}

func (c *ClientSet) EventV1() *eventv1.ClientV1 {
	return c.eventV1Client
}

func (c *ClientSet) LogV1() *logv1.ClientV1 {
	return c.logV1Client
}

func (c *ClientSet) VolumeV1() volumev1.ClientV1 {
	return c.volumeV1Client
}

func (c *ClientSet) State() connectivity.State {
	return c.conn.GetState()
}

func (c *ClientSet) Connect() {
	c.conn.Connect()
}

func (c *ClientSet) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	defer func() {
		if err := c.conn.Close(); err != nil {
			c.logger.Error("error closing connection", "error", err)
		}
	}()
	return nil
}

func (c *ClientSet) ID() string {
	return c.clientId
}

func New(server string, opts ...NewClientOption) (*ClientSet, error) {
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
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	}

	// Default clientset
	c := &ClientSet{
		grpcOpts: defaultOpts,
		tlsConfig: &tls.Config{
			InsecureSkipVerify: false,
		},
		clientId: uuid.New().String(),
		logger:   logger.ConsoleLogger{},
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

	conn, err := grpc.NewClient(server, c.grpcOpts...)
	if err != nil {
		return nil, err
	}

	c.conn = conn
	c.NodeV1Client = nodev1.NewClientV1WithConn(conn, c.clientId, nodev1.WithLogger(c.logger))
	c.ContainerV1Client = containerv1.NewClientV1WithConn(conn, c.clientId)
	c.containerSetV1Client = containersetv1.NewClientV1(conn, c.clientId)
	c.eventV1Client = eventv1.NewClientV1(conn, c.clientId)
	c.logV1Client = logv1.NewClientV1(conn)
	c.volumeV1Client = volumev1.NewClientV1WithConn(conn, c.clientId)

	return c, nil
}
