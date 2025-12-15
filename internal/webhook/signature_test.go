package webhook

import (
	"strings"
	"testing"
)

func TestComputeHMAC(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		secret  string
	}{
		{
			name:    "simple payload",
			payload: "hello world",
			secret:  "my-secret",
		},
		{
			name:    "empty payload",
			payload: "",
			secret:  "my-secret",
		},
		{
			name:    "json payload",
			payload: `{"key":"value"}`,
			secret:  "secret123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ComputeHMAC([]byte(tt.payload), tt.secret)
			// Check that result has correct format
			if !strings.HasPrefix(result, "sha256=") {
				t.Errorf("ComputeHMAC() result does not have 'sha256=' prefix: %v", result)
			}
			// Check that hex encoding is 64 chars (32 bytes = 256 bits)
			hexPart := strings.TrimPrefix(result, "sha256=")
			if len(hexPart) != 64 {
				t.Errorf("ComputeHMAC() hex part length = %v, want 64", len(hexPart))
			}
		})
	}
}

func TestVerifySignature(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		secret  string
		want    bool
	}{
		{
			name:    "valid signature",
			payload: "hello world",
			secret:  "my-secret",
			want:    true,
		},
		{
			name:    "wrong secret",
			payload: "hello world",
			secret:  "wrong-secret",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Compute the signature with the test secret
			var signature string
			if tt.want {
				signature = ComputeHMAC([]byte(tt.payload), tt.secret)
			} else {
				// Use signature computed with different secret
				signature = ComputeHMAC([]byte(tt.payload), "different-secret")
			}
			
			result := VerifySignature([]byte(tt.payload), signature, tt.secret)
			if result != tt.want {
				t.Errorf("VerifySignature() = %v, want %v", result, tt.want)
			}
		})
	}

	// Test invalid signature format
	t.Run("invalid signature", func(t *testing.T) {
		result := VerifySignature([]byte("hello world"), "sha256=invalid", "my-secret")
		if result {
			t.Errorf("VerifySignature() with invalid signature should return false")
		}
	})

	// Test empty signature
	t.Run("empty signature", func(t *testing.T) {
		result := VerifySignature([]byte("hello world"), "", "my-secret")
		if result {
			t.Errorf("VerifySignature() with empty signature should return false")
		}
	})
}

func TestGenerateSecret(t *testing.T) {
	// Test that secret generation works
	secret1, err := GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret() error = %v", err)
	}

	// Check that secret has the correct prefix
	if !strings.HasPrefix(secret1, "whsec_") {
		t.Errorf("GenerateSecret() secret does not have 'whsec_' prefix: %v", secret1)
	}

	// Check that secret has reasonable length (prefix + base64 encoded 32 bytes)
	if len(secret1) < 20 {
		t.Errorf("GenerateSecret() secret too short: %v", len(secret1))
	}

	// Generate another secret and ensure they're different
	secret2, err := GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret() error = %v", err)
	}

	if secret1 == secret2 {
		t.Errorf("GenerateSecret() generated identical secrets, should be random")
	}
}

func TestSignatureRoundTrip(t *testing.T) {
	// Test that we can sign and verify a payload
	payload := []byte(`{"event":"flag.updated","timestamp":"2025-01-15T10:30:00Z"}`)
	secret, err := GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret() error = %v", err)
	}

	signature := ComputeHMAC(payload, secret)
	if !VerifySignature(payload, signature, secret) {
		t.Errorf("Failed to verify signature that was just computed")
	}
}
