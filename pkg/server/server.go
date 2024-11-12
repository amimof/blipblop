package server

import (
	"context"
	"io"
	"net"

	"github.com/amimof/blipblop/pkg/logger"
	"github.com/amimof/blipblop/services"

	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/keepalive"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
)

func init() {
	grpclog.SetLoggerV2(grpclog.NewLoggerV2(io.Discard, io.Discard, io.Discard))
}

type Server struct {
	grpcOpts   []grpc.ServerOption
	grpcServer *grpc.Server
	// addr       *string
	logger logger.Logger
	tracer *trace.TracerProvider
}

type NewServerOption func(*Server)

func WithLogger(lgr logger.Logger) NewServerOption {
	return func(s *Server) {
		s.logger = lgr
	}
}

func WithGrpcOption(opts ...grpc.ServerOption) NewServerOption {
	return func(s *Server) {
		s.grpcOpts = append(s.grpcOpts, opts...)
	}
}

func New(opts ...NewServerOption) (*Server, error) {
	grpcOpts := []grpc.ServerOption{
		grpc.KeepaliveParams(keepalive.ServerParameters{}),
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	}

	// Setup server
	server := &Server{
		grpcOpts: grpcOpts,
		// addr:     &addr,
		logger: logger.ConsoleLogger{},
	}

	// Setup tracing
	exporter, err := otlptracehttp.New(context.Background(), otlptracehttp.WithInsecure(), otlptracehttp.WithEndpoint("192.168.13.123:4318"))
	if err != nil {
		return nil, err
	}
	server.tracer = trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("blipblop"),
		)))
	otel.SetTracerProvider(server.tracer)

	// Apply options
	for _, opt := range opts {
		opt(server)
	}

	server.grpcServer = grpc.NewServer(server.grpcOpts...)

	return server, nil
}

func (s *Server) Serve(lis net.Listener) error {
	return s.grpcServer.Serve(lis)
}

func (s *Server) Shutdown() {
	s.tracer.Shutdown(context.Background())
	s.grpcServer.GracefulStop()
}

func (s *Server) ForceShutdown() {
	s.tracer.Shutdown(context.Background())
	s.grpcServer.Stop()
}

func (s *Server) RegisterService(svcs ...services.Service) error {
	for _, svc := range svcs {
		err := svc.Register(s.grpcServer)
		if err != nil {
			return err
		}
	}
	return nil
}
