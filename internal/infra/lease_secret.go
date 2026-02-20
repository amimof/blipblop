package infra

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// Generate a secure random refresh token (512-bit)
func GenerateHMAC(message, secret []byte) (string, error) {
	h := hmac.New(sha256.New, secret)
	_, err := h.Write(message)
	if err != nil {
		return "", err
	}
	hash := h.Sum(nil)

	// URL-safe, no padding
	return base64.RawURLEncoding.EncodeToString(hash), nil
}

// Generate a secure random refresh token (512-bit)
func GenerateToken() (string, error) {
	data := make([]byte, 64)
	_, err := rand.Read(data)
	if err != nil {
		return "", err
	}
	// URL-safe, no padding
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func ValidateHMAC(message, expectedHash []byte, secret []byte) (bool, error) {
	h, err := GenerateHMAC(message, secret)
	if err != nil {
		return false, err
	}
	return hmac.Equal([]byte(h), expectedHash), nil
}
