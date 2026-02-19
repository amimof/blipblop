package app

import (
	"go.opentelemetry.io/otel"
)

var tracer = otel.GetTracerProvider().Tracer("voiyd-server")

type Version string

var (
	VersionLeaseV1 Version = "lease/v1"
	VersionEventV1 Version = "event/v1"
)
