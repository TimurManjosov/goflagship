package auth

import (
	"context"
	"net/http"
	"time"

	"github.com/TimurManjosov/goflagship/internal/db/gen"
	"github.com/jackc/pgx/v5/pgtype"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	// ContextKeyAPIKey is the context key for storing the API key ID
	ContextKeyAPIKey contextKey = "api_key_id"
	// ContextKeyRole is the context key for storing the user role
	ContextKeyRole contextKey = "role"
)

// KeyStore defines the interface for API key storage operations
type KeyStore interface {
	ListAPIKeys(ctx context.Context) ([]dbgen.ApiKey, error)
	UpdateAPIKeyLastUsed(ctx context.Context, id pgtype.UUID) error
}

// Authenticator handles authentication for API requests
type Authenticator struct {
	keyStore       KeyStore
	legacyAdminKey string // For backward compatibility
}

// NewAuthenticator creates a new Authenticator
func NewAuthenticator(keyStore KeyStore, legacyAdminKey string) *Authenticator {
	return &Authenticator{
		keyStore:       keyStore,
		legacyAdminKey: legacyAdminKey,
	}
}

// AuthResult contains the result of an authentication attempt
type AuthResult struct {
	Authenticated bool
	Role          Role
	APIKeyID      pgtype.UUID
	Error         string
}

// Authenticate authenticates a request using the Authorization header
// It supports both legacy ADMIN_API_KEY and database-stored API keys
func (a *Authenticator) Authenticate(ctx context.Context, authHeader string) AuthResult {
	// Extract bearer token
	token := ExtractBearerToken(authHeader)
	if token == "" {
		return AuthResult{
			Authenticated: false,
			Error:         "missing bearer token",
		}
	}

	// First, try legacy admin key (for backward compatibility)
	if a.legacyAdminKey != "" && VerifyAPIKeyConstantTime(token, a.legacyAdminKey) {
		return AuthResult{
			Authenticated: true,
			Role:          RoleSuperadmin,
		}
	}

	// Try database-stored keys
	// We need to query all enabled keys and verify each hash
	// This is the only way with bcrypt since hashes are non-deterministic
	// For better performance, consider implementing a cache layer
	keys, err := a.keyStore.ListAPIKeys(ctx)
	if err != nil {
		return AuthResult{
			Authenticated: false,
			Error:         "authentication service unavailable",
		}
	}

	var apiKey *dbgen.ApiKey
	for i := range keys {
		if keys[i].Enabled && VerifyAPIKey(token, keys[i].KeyHash) {
			apiKey = &keys[i]
			break
		}
	}

	if apiKey == nil {
		return AuthResult{
			Authenticated: false,
			Error:         "invalid token",
		}
	}

	// Check if key is expired
	if apiKey.ExpiresAt.Valid {
		expiresAt := apiKey.ExpiresAt.Time
		if time.Now().After(expiresAt) {
			return AuthResult{
				Authenticated: false,
				Error:         "api key expired",
			}
		}
	}

	// Update last used timestamp (async, ignore errors)
	go func() {
		_ = a.keyStore.UpdateAPIKeyLastUsed(context.Background(), apiKey.ID)
	}()

	return AuthResult{
		Authenticated: true,
		Role:          Role(apiKey.Role),
		APIKeyID:      apiKey.ID,
	}
}

// RequireAuth is a middleware that requires authentication
func (a *Authenticator) RequireAuth(requiredRole Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			result := a.Authenticate(r.Context(), authHeader)

			if !result.Authenticated {
				http.Error(w, result.Error, http.StatusUnauthorized)
				return
			}

			// Check if user has required permission
			if !HasPermission(result.Role, requiredRole) {
				http.Error(w, "insufficient permissions", http.StatusForbidden)
				return
			}

			// Add auth info to context
			ctx := context.WithValue(r.Context(), ContextKeyRole, result.Role)
			if result.APIKeyID.Valid {
				ctx = context.WithValue(ctx, ContextKeyAPIKey, result.APIKeyID)
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetRoleFromContext extracts the role from the request context
func GetRoleFromContext(ctx context.Context) (Role, bool) {
	role, ok := ctx.Value(ContextKeyRole).(Role)
	return role, ok
}

// GetAPIKeyIDFromContext extracts the API key ID from the request context
func GetAPIKeyIDFromContext(ctx context.Context) (pgtype.UUID, bool) {
	id, ok := ctx.Value(ContextKeyAPIKey).(pgtype.UUID)
	return id, ok
}
