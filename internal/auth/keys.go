package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const (
	// KeyPrefix is the prefix for all generated API keys
	KeyPrefix = "fsk_"
	// KeyLength is the length of the random part of the key (32 bytes = 256 bits)
	KeyLength = 32
	// BCryptCost is the cost factor for bcrypt hashing
	BCryptCost = 12
)

// Role represents the access level of an API key
type Role string

const (
	RoleReadonly   Role = "readonly"
	RoleAdmin      Role = "admin"
	RoleSuperadmin Role = "superadmin"
)

// GenerateAPIKey generates a new API key with the given prefix
func GenerateAPIKey() (string, error) {
	// Generate random bytes
	randomBytes := make([]byte, KeyLength)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Encode to base64 (URL-safe, no padding)
	encoded := base64.RawURLEncoding.EncodeToString(randomBytes)

	// Return with prefix
	return KeyPrefix + encoded, nil
}

// HashAPIKey hashes an API key using bcrypt
func HashAPIKey(key string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(key), BCryptCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash key: %w", err)
	}
	return string(hash), nil
}

// VerifyAPIKey verifies an API key against a hash using constant-time comparison
func VerifyAPIKey(key, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(key))
	return err == nil
}

// VerifyAPIKeyConstantTime verifies an API key against a plain text key using constant-time comparison
// This is used for the legacy ADMIN_API_KEY environment variable
func VerifyAPIKeyConstantTime(got, expected string) bool {
	return subtle.ConstantTimeCompare([]byte(got), []byte(expected)) == 1
}

// ExtractBearerToken extracts the bearer token from an Authorization header
func ExtractBearerToken(authHeader string) string {
	// Remove "Bearer " prefix (case-insensitive)
	token := strings.TrimSpace(authHeader)
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		token = strings.TrimSpace(token[7:])
	}
	return token
}

// ValidateRole checks if a given role string is valid
func ValidateRole(role string) bool {
	switch Role(role) {
	case RoleReadonly, RoleAdmin, RoleSuperadmin:
		return true
	default:
		return false
	}
}

// HasPermission checks if a given role has permission to access a resource
// readonly: can only read
// admin: can read and write (but not manage keys)
// superadmin: can do everything including key management
func HasPermission(userRole Role, requiredRole Role) bool {
	// superadmin can do everything
	if userRole == RoleSuperadmin {
		return true
	}

	// admin can do admin and readonly operations
	if userRole == RoleAdmin && (requiredRole == RoleAdmin || requiredRole == RoleReadonly) {
		return true
	}

	// readonly can only do readonly operations
	if userRole == RoleReadonly && requiredRole == RoleReadonly {
		return true
	}

	return false
}
