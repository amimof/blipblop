package app

import (
	"github.com/amimof/voiyd/pkg/errs"
	"go.opentelemetry.io/otel"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var tracer = otel.GetTracerProvider().Tracer("voiyd-server")

type Version string

var (
	VersionLeaseV1 Version = "lease/v1"
	VersionEventV1 Version = "event/v1"
)

func mapError(err error) error {
	if errs.IsNotFound(err) {
		return status.Error(codes.NotFound, err.Error())
	}
	return status.Error(codes.Internal, err.Error())
}
