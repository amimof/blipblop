// Package errs provides convenient constructs to work with errors
package errs

import (
	"errors"
	"os"

	"github.com/amimof/voiyd/pkg/repository"
	"github.com/containerd/containerd/errdefs"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var ErrLeaseHeld = errors.New("lease: already held by another holder")

// ErrLeaseNotFound = errors.New("lease: not found")
// ErrLeaseExpired  = errors.New("lease: expired")
// ErrInvalidTTL    = errors.New("lease: invalid ttl")
// ErrInvalidHolder = errors.New("lease: invalid holder")

func IsConflict(err error) bool {
	if err == nil {
		return false
	}

	switch {
	case errors.Is(err, ErrLeaseHeld):
		return true
	case errors.Is(err, repository.ErrIdxExists):
		return true
	}

	return false
}

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

	// os errors
	if errors.Is(err, os.ErrNotExist) {
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
