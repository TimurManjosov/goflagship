package auth

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
)

// AuditLogger defines the interface for audit logging operations
type AuditLogger interface {
	CreateAuditLog(ctx context.Context, apiKeyID pgtype.UUID, action, resource, ipAddress, userAgent string, status int32, details map[string]interface{}) error
}

// AuditEntry represents an audit log entry
type AuditEntry struct {
	APIKeyID  pgtype.UUID
	Action    string
	Resource  string
	IPAddress string
	UserAgent string
	Status    int
	Details   map[string]interface{}
}

// LogAudit logs an audit entry
func LogAudit(ctx context.Context, logger AuditLogger, entry AuditEntry) error {
	detailsJSON, err := json.Marshal(entry.Details)
	if err != nil {
		detailsJSON = []byte("{}")
	}

	var details map[string]interface{}
	if err := json.Unmarshal(detailsJSON, &details); err != nil {
		details = make(map[string]interface{})
	}

	return logger.CreateAuditLog(
		ctx,
		entry.APIKeyID,
		entry.Action,
		entry.Resource,
		entry.IPAddress,
		entry.UserAgent,
		int32(entry.Status),
		details,
	)
}

// GetIPAddress extracts the IP address from the request
func GetIPAddress(r *http.Request) string {
	// Check X-Forwarded-For header first (for proxies)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the list
		return xff
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	return r.RemoteAddr
}
