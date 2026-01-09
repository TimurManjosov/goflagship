package auth

import (
	"context"
	"net/http"
	"sync/atomic"
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

// lastUsedUpdate represents a request to update the last_used_at timestamp
type lastUsedUpdate struct {
	id pgtype.UUID
}

// Authenticator handles authentication for API requests
type Authenticator struct {
	keyStore       KeyStore
	legacyAdminKey string // For backward compatibility
	updateChan     chan lastUsedUpdate
	closed         int32 // atomic flag to prevent double-close
}

// NewAuthenticator creates a new Authenticator with a background worker
func NewAuthenticator(keyStore KeyStore, legacyAdminKey string) *Authenticator {
	auth := &Authenticator{
		keyStore:       keyStore,
		legacyAdminKey: legacyAdminKey,
		updateChan:     make(chan lastUsedUpdate, 100), // Buffered channel to prevent blocking
	}

	// Start background worker for updating last_used_at timestamps
	go auth.lastUsedWorker()

	return auth
}

// lastUsedWorker processes last_used_at updates in the background.
// It runs until the updateChan is closed.
func (a *Authenticator) lastUsedWorker() {
	for update := range a.updateChan {
		// Skip if keyStore is nil
		if a.keyStore == nil {
			continue
		}
		// Use a background context with timeout to prevent hanging
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = a.keyStore.UpdateAPIKeyLastUsed(ctx, update.id)
		cancel()
	}
}

// Close gracefully shuts down the authenticator by closing the update channel.
// This causes the background worker to exit after processing any pending updates.
// After Close is called, the Authenticator should not be used for new authentication requests.
//
// Close is safe to call multiple times - subsequent calls are no-ops.
func (a *Authenticator) Close() error {
	// Atomically check if already closed
	if !atomic.CompareAndSwapInt32(&a.closed, 0, 1) {
		return nil // Already closed
	}
	// Close channel to signal worker to stop
	close(a.updateChan)
	return nil
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
	// Note: This queries all enabled keys and verifies each hash with bcrypt
	// This is necessary because bcrypt hashes are non-deterministic (include random salt)
	// For production deployments with many keys, consider implementing a cache layer:
	// - Cache keys for 5-10 minutes with periodic refresh
	// - Invalidate cache on key creation/revocation
	// - Use a mutex or RWMutex for thread-safe cache access
	if a.keyStore == nil {
		return AuthResult{
			Authenticated: false,
			Error:         "invalid token",
		}
	}

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

	// Update last used timestamp (non-blocking, handled by background worker)
	if apiKey.ID.Valid {
		select {
		case a.updateChan <- lastUsedUpdate{id: apiKey.ID}:
			// Successfully queued
		default:
			// Channel full, skip this update (acceptable trade-off)
		}
	}

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
