package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/golang-jwt/jwt/v5"
)

type ctxKey string

type AuthInterceptor struct {
	accessibleRoles map[string][]string
	pubKey          ecdsa.PublicKey
}

func HashRefreshToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

func GenerateRefreshToken() (string, string, error) {
	b := make([]byte, 32)

	_, err := rand.Read(b)
	if err != nil {
		return "", "", err
	}

	token := base64.RawURLEncoding.EncodeToString(b)
	hash := sha256.Sum256([]byte(token))
	tokenHash := base64.RawURLEncoding.EncodeToString(hash[:])
	return token, tokenHash, nil
}

func Generate(claims jwt.Claims, key *ecdsa.PrivateKey) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	return token.SignedString(key)
}

func Verify(accessToken string, key ecdsa.PublicKey) (*jwt.Token, error) {
	token, err := jwt.ParseWithClaims(
		accessToken,
		&LeaseClaims{},
		func(token *jwt.Token) (any, error) {
			return key, nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("token validation failed: %w", err)
	}

	return token, nil
}

func NewAuthInterceptor(accessibleRoles map[string][]string, pubKey ecdsa.PublicKey) *AuthInterceptor {
	return &AuthInterceptor{accessibleRoles, pubKey}
}

func (a *AuthInterceptor) authorize(ctx context.Context, method string) (*LeaseClaims, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "metadata is not provided")
	}

	values := md["authorization"]
	if len(values) == 0 {
		return nil, status.Errorf(codes.Unauthenticated, "authorization token is not provided")
	}

	accessToken := values[0]
	claims, err := Verify(accessToken, a.pubKey)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "access token is invalid: %v", err)
	}

	leaseClaims, ok := claims.Claims.(*LeaseClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	return leaseClaims, nil
}

func (a *AuthInterceptor) Unary() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		claims, err := a.authorize(ctx, info.FullMethod)
		if err != nil {
			return nil, err
		}

		if claims != nil {
			ctx = context.WithValue(ctx, ctxKey("lease"), claims.LeaseID)
			ctx = context.WithValue(ctx, ctxKey("subject"), claims.Subject)
			ctx = context.WithValue(ctx, ctxKey("resource"), claims.Resource)
		}

		return handler(ctx, req)
	}
}

func (a *AuthInterceptor) Stream() grpc.StreamServerInterceptor {
	return func(
		srv any,
		stream grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		// ctx := stream.Context()
		// claims, err := a.authorize(ctx, info.FullMethod)
		// if err != nil {
		// 	return err
		// }
		// if claims != nil {
		// 	ctx = context.WithValue(ctx, ctxKey("lease"), claims.LeaseID)
		// 	ctx = context.WithValue(ctx, ctxKey("subject"), claims.Subject)
		// 	ctx = context.WithValue(ctx, ctxKey("resource"), claims.Resource)
		// }
		//
		return handler(srv, stream)
	}
}
