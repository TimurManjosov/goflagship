package auth

import (
	"strings"
	"testing"
)

func TestGenerateAPIKey(t *testing.T) {
	key, err := GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey() error = %v", err)
	}

	// Check prefix
	if !strings.HasPrefix(key, KeyPrefix) {
		t.Errorf("GenerateAPIKey() = %v, want prefix %v", key, KeyPrefix)
	}

	// Check length (prefix + base64-encoded 32 bytes)
	// Base64 URL encoding without padding: 32 bytes -> 43 characters
	expectedLen := len(KeyPrefix) + 43
	if len(key) != expectedLen {
		t.Errorf("GenerateAPIKey() length = %v, want %v", len(key), expectedLen)
	}
}

func TestHashAndVerifyAPIKey(t *testing.T) {
	key := "test-api-key-12345"

	// Hash the key
	hash, err := HashAPIKey(key)
	if err != nil {
		t.Fatalf("HashAPIKey() error = %v", err)
	}

	// Verify correct key
	if !VerifyAPIKey(key, hash) {
		t.Error("VerifyAPIKey() failed for correct key")
	}

	// Verify incorrect key
	if VerifyAPIKey("wrong-key", hash) {
		t.Error("VerifyAPIKey() succeeded for incorrect key")
	}
}

func TestVerifyAPIKeyConstantTime(t *testing.T) {
	tests := []struct {
		name     string
		got      string
		expected string
		want     bool
	}{
		{"equal", "admin-123", "admin-123", true},
		{"not equal", "admin-456", "admin-123", false},
		{"empty got", "", "admin-123", false},
		{"empty expected", "admin-123", "", false},
		{"both empty", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := VerifyAPIKeyConstantTime(tt.got, tt.expected); got != tt.want {
				t.Errorf("VerifyAPIKeyConstantTime() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name       string
		authHeader string
		want       string
	}{
		{"with Bearer prefix", "Bearer token123", "token123"},
		{"with bearer lowercase", "bearer token456", "token456"},
		{"with extra spaces", "Bearer  token789  ", "token789"},
		{"without Bearer prefix", "token999", "token999"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractBearerToken(tt.authHeader); got != tt.want {
				t.Errorf("ExtractBearerToken() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateRole(t *testing.T) {
	tests := []struct {
		name string
		role string
		want bool
	}{
		{"readonly", "readonly", true},
		{"admin", "admin", true},
		{"superadmin", "superadmin", true},
		{"invalid", "invalid", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidateRole(tt.role); got != tt.want {
				t.Errorf("ValidateRole() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasPermission(t *testing.T) {
	tests := []struct {
		name         string
		userRole     Role
		requiredRole Role
		want         bool
	}{
		{"superadmin can do everything", RoleSuperadmin, RoleSuperadmin, true},
		{"superadmin can do admin", RoleSuperadmin, RoleAdmin, true},
		{"superadmin can do readonly", RoleSuperadmin, RoleReadonly, true},
		{"admin can do admin", RoleAdmin, RoleAdmin, true},
		{"admin can do readonly", RoleAdmin, RoleReadonly, true},
		{"admin cannot do superadmin", RoleAdmin, RoleSuperadmin, false},
		{"readonly can do readonly", RoleReadonly, RoleReadonly, true},
		{"readonly cannot do admin", RoleReadonly, RoleAdmin, false},
		{"readonly cannot do superadmin", RoleReadonly, RoleSuperadmin, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasPermission(tt.userRole, tt.requiredRole); got != tt.want {
				t.Errorf("HasPermission() = %v, want %v", got, tt.want)
			}
		})
	}
}
