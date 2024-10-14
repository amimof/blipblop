package errors

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
