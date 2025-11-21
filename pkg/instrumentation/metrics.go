package instrumentation

import (
	"context"

	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
	"google.golang.org/grpc"
	"google.golang.org/grpc/stats/opentelemetry"
)

// type Shutdown func(context.Context) error

func InitServerMetrics(ctx context.Context) (grpc.ServerOption, error) {
	exporter, err := prometheus.New()
	if err != nil {
		return nil, err
	}

	meterProvider := metric.NewMeterProvider(metric.WithReader(exporter))

	opts := opentelemetry.ServerOption(opentelemetry.Options{
		MetricsOptions: opentelemetry.MetricsOptions{
			MeterProvider: meterProvider,
		},
	})

	return opts, nil
}

func InitClientMetrics() (grpc.DialOption, error) {
	exporter, err := prometheus.New()
	if err != nil {
		return nil, err
	}
	meterProvider := metric.NewMeterProvider(metric.WithReader(exporter))

	opts := opentelemetry.DialOption(opentelemetry.Options{
		MetricsOptions: opentelemetry.MetricsOptions{
			MeterProvider: meterProvider,
		},
	})

	return opts, nil
}
