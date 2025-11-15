// Package errors provides convenient constructs to work with errors
package errors

import (
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
	return b
}

func IgnoreNotFound(err error) error {
	if IsNotFound(err) {
		return nil
	}
	return err
}
