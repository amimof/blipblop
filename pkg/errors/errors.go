// Package errors provides convenient constructs to work with errors
package errors

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	ERRIMAGEPULL    = "ErrPulling"
	ERREESCHEDULING = "ErrScheduling"
	ERREXEC         = "ErrExec"
	ERRDELETE       = "ErrDeleting"
	ERRSTOP         = "ErrStopping"
	ERRKILL         = "ErrKilling"
)

func IsNotFound(err error) bool {
	var b bool
	if st, ok := status.FromError(err); ok {
		if st.Code() == codes.NotFound {
			b = true
		}
	}
	return b
}

func IgnoreNotFound(err error) error {
	if IsNotFound(err) {
		return nil
	}
	return err
}
