package server

import (
	"context"
	"net"
	"net/http"

	"github.com/amimof/blipblop/api/services/containers/v1"
	"github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/api/services/nodes/v1"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
)

var DefaultMux = runtime.NewServeMux(runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{MarshalOptions: protojson.MarshalOptions{EmitUnpopulated: true}}))

type Gateway struct {
	mux  *runtime.ServeMux
	srv  *http.Server
	conn *grpc.ClientConn
}

func (g *Gateway) Serve(lis net.Listener) error {
	return g.srv.Serve(lis)
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

func NewGateway(ctx context.Context, addr string, mux *runtime.ServeMux) (*Gateway, error) {
	conn, err := grpc.DialContext(
		ctx,
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, err
	}
	err = events.RegisterEventServiceHandler(ctx, mux, conn)
	if err != nil {
		return nil, err
	}
	err = containers.RegisterContainerServiceHandler(ctx, mux, conn)
	if err != nil {
		return nil, err
	}
	err = nodes.RegisterNodeServiceHandler(ctx, mux, conn)
	if err != nil {
		return nil, err
	}
	return &Gateway{
		mux:  mux,
		conn: conn,
		srv: &http.Server{
			Addr:    addr,
			Handler: mux,
		},
	}, nil
}
