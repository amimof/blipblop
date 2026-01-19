// Package errors provides convenient constructs to work with errors
package errors

import (
	"errors"

	"github.com/amimof/voiyd/pkg/repository"
	"github.com/containerd/containerd/errdefs"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func IsNotFound(err error) bool {
	var b bool

	// grpc errors
	if st, ok := status.FromError(err); ok {
		if st.Code() == codes.NotFound {
			b = true
		}
	}

	// containerd errors
	if errdefs.IsNotFound(err) {
		b = true
	}

	// repo errors
	if errors.Is(err, repository.ErrNotFound) {
		b = true
	}

	return b
}

func IgnoreNotFound(err error) error {
	if IsNotFound(err) {
		return nil
	}
	return err
}
