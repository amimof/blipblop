package server

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"

	containersetsv1 "github.com/amimof/voiyd/api/services/containersets/v1"
	leasesv1 "github.com/amimof/voiyd/api/services/leases/v1"
	nodesv1 "github.com/amimof/voiyd/api/services/nodes/v1"
	tasksv1 "github.com/amimof/voiyd/api/services/tasks/v1"
	volumesv1 "github.com/amimof/voiyd/api/services/volumes/v1"
)

var DefaultMux = runtime.NewServeMux(runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{MarshalOptions: protojson.MarshalOptions{EmitUnpopulated: true}}))

type Gateway struct {
	mux      *runtime.ServeMux
	srv      *http.Server
	conn     *grpc.ClientConn
	grpcOpts []grpc.DialOption
}

type NewGatewayOption func(g *Gateway)

func WithGrpcDialOption(opts ...grpc.DialOption) NewGatewayOption {
	return func(g *Gateway) {
		g.grpcOpts = opts
	}
}

func WithTLSConfig(t *tls.Config) NewGatewayOption {
	return func(g *Gateway) {
		g.srv.TLSConfig = t
	}
}

func (g *Gateway) Serve(lis net.Listener) error {
	return g.srv.Serve(lis)
}

func (g *Gateway) ServeTLS(lis net.Listener, certFile, keyFile string) error {
	return g.srv.ServeTLS(lis, certFile, keyFile)
}

func (g *Gateway) Shutdown(ctx context.Context) error {
	if g.conn != nil {
		return g.conn.Close()
	}
	if g.srv != nil {
		return g.srv.Shutdown(ctx)
	}
	return nil
}

func NewGateway(ctx context.Context, addr string, mux *runtime.ServeMux, opts ...NewGatewayOption) (*Gateway, error) {
	grpcOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	g := &Gateway{
		mux: mux,
		srv: &http.Server{
			Addr:    addr,
			Handler: mux,
		},
		grpcOpts: grpcOpts,
	}

	for _, opt := range opts {
		opt(g)
	}

	conn, err := grpc.NewClient(
		addr,
		g.grpcOpts...,
	)
	if err != nil {
		return nil, err
	}
	err = containersetsv1.RegisterContainerSetServiceHandler(ctx, mux, conn)
	if err != nil {
		return nil, err
	}
	err = tasksv1.RegisterTaskServiceHandler(ctx, mux, conn)
	if err != nil {
		return nil, err
	}
	err = nodesv1.RegisterNodeServiceHandler(ctx, mux, conn)
	if err != nil {
		return nil, err
	}
	err = volumesv1.RegisterVolumeServiceHandler(ctx, mux, conn)
	if err != nil {
		return nil, err
	}
	err = leasesv1.RegisterLeaseServiceHandler(ctx, mux, conn)
	if err != nil {
		return nil, err
	}
	g.conn = conn

	return g, nil
}
