package webhook

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// ComputeHMAC generates an HMAC signature for the given payload using the secret
func ComputeHMAC(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// VerifySignature verifies that the provided signature matches the computed HMAC
func VerifySignature(payload []byte, signature string, secret string) bool {
	expected := ComputeHMAC(payload, secret)
	return hmac.Equal([]byte(signature), []byte(expected))
}

// GenerateSecret generates a cryptographically secure random secret for webhook signing
func GenerateSecret() (string, error) {
	// Generate 32 bytes of random data
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random secret: %w", err)
	}
	// Return base64-encoded string with "whsec_" prefix (webhook secret)
	return "whsec_" + base64.URLEncoding.EncodeToString(bytes), nil
}
