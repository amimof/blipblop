package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type LeaseClaims struct {
	Resource     string   `json:"resource"`
	Scope        []string `json:"scope"`
	LeaseID      string   `json:"lease_id"`
	FencingToken uint64   `json:"fencing_token"`
	jwt.RegisteredClaims
}

func NewLeaseClaim(taskID, nodeID, leaseID string, expiresAt time.Time) *LeaseClaims {
	return &LeaseClaims{
		Resource:     taskID,
		LeaseID:      leaseID,
		FencingToken: 1,
		Scope:        []string{"task.status.write"},
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "lease-service",
			Subject:   nodeID,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
}
