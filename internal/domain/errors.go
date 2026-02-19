package domain

import "errors"

var (
	ErrNotHolder     = errors.New("lease: caller is not the holder")
	ErrLeaseHeld     = errors.New("lease: already held by another holder")
	ErrLeaseNotFound = errors.New("lease: not found")
	ErrLeaseExpired  = errors.New("lease: expired")
	ErrInvalidTTL    = errors.New("lease: invalid ttl")
	ErrInvalidHolder = errors.New("lease: invalid holder")
)
