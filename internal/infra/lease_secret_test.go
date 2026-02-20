package infra

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHMAC(t *testing.T) {
	secret := []byte("secretkey")
	token, err := GenerateToken()
	if err != nil {
		t.Fatal(err)
	}

	hashedToken, err := GenerateHMAC([]byte(token), secret)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Token: %s, HashedToken: %s", token, hashedToken)

	valid, err := ValidateHMAC([]byte(token), []byte(hashedToken), secret)
	if err != nil {
		t.Fatal(err)
	}

	assert.True(t, valid)
}
