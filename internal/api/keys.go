package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/TimurManjosov/goflagship/internal/audit"
	"github.com/TimurManjosov/goflagship/internal/auth"
	dbgen "github.com/TimurManjosov/goflagship/internal/db/gen"
	"github.com/TimurManjosov/goflagship/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// --- API Key Management Endpoints ---

type createKeyRequest struct {
	Name      string  `json:"name"`
	Role      string  `json:"role"`
	ExpiresAt *string `json:"expires_at,omitempty"` // ISO 8601 format
}

type createKeyResponse struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Key       string  `json:"key"` // Only shown once!
	Role      string  `json:"role"`
	CreatedAt string  `json:"created_at"`
	ExpiresAt *string `json:"expires_at,omitempty"`
}

type listKeysResponse struct {
	Keys []keyInfo `json:"keys"`
}

type keyInfo struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Role       string  `json:"role"`
	Enabled    bool    `json:"enabled"`
	CreatedAt  string  `json:"created_at"`
	LastUsedAt *string `json:"last_used_at,omitempty"`
	ExpiresAt  *string `json:"expires_at,omitempty"`
}

// handleCreateAPIKey creates a new API key (superadmin only)
func (s *Server) handleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	// Limit request body size to prevent memory exhaustion attacks
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB limit

	var req createKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if err.Error() == "http: request body too large" {
			RequestTooLargeError(w, r, "Request body too large")
			return
		}
		BadRequestError(w, r, ErrCodeInvalidJSON, "Invalid JSON: expected fields 'name', 'role', and optional 'expires_at'")
		return
	}

	// Validate all fields at once
	validationErrors := make(map[string]string)

	if req.Name == "" {
		validationErrors["name"] = "Name is required"
	}

	if !auth.ValidateRole(req.Role) {
		validationErrors["role"] = "Role must be readonly, admin, or superadmin"
	}

	// Parse expires_at if provided
	var expiresAt pgtype.Timestamptz
	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			validationErrors["expires_at"] = "Invalid format: use ISO 8601 (e.g., 2024-12-31T23:59:59Z)"
		} else {
			expiresAt = pgtype.Timestamptz{Time: t, Valid: true}
		}
	}

	// Return all validation errors at once
	if len(validationErrors) > 0 {
		ValidationError(w, r, "Validation failed for one or more fields", validationErrors)
		return
	}

	// Generate new API key
	key, err := auth.GenerateAPIKey()
	if err != nil {
		InternalError(w, r, "Failed to generate key")
		return
	}

	// Hash the key
	keyHash, err := auth.HashAPIKey(key)
	if err != nil {
		InternalError(w, r, "Failed to hash key")
		return
	}

	// Get creator info from context
	createdBy := "legacy-admin"
	if apiKeyID, ok := auth.GetAPIKeyIDFromContext(r.Context()); ok && apiKeyID.Valid {
		createdBy = fmt.Sprintf("%x", apiKeyID.Bytes[:8]) // Use first 8 bytes of UUID as identifier
	}

	// Create the key in database
	pgStore, ok := s.store.(PostgresStoreInterface)
	if !ok {
		InternalError(w, r, "Database store not available")
		return
	}

	apiKey, err := pgStore.CreateAPIKey(r.Context(), dbgen.CreateAPIKeyParams{
		Name:      req.Name,
		KeyHash:   keyHash,
		Role:      dbgen.ApiKeyRole(req.Role),
		Enabled:   true,
		ExpiresAt: expiresAt,
		CreatedBy: createdBy,
	})
	if err != nil {
		InternalError(w, r, "Failed to create key")
		return
	}

	// Log the action (using new audit service)
	afterState := map[string]any{
		"id":       uuidToString(apiKey.ID),
		"name":     apiKey.Name,
		"role":     string(apiKey.Role),
		"enabled":  apiKey.Enabled,
		"created_at": apiKey.CreatedAt.Time.Format(time.RFC3339),
	}
	if apiKey.ExpiresAt.Valid {
		afterState["expires_at"] = apiKey.ExpiresAt.Time.Format(time.RFC3339)
	}
	s.auditLog(r, audit.ActionCreated, audit.ResourceTypeAPIKey, uuidToString(apiKey.ID), "", nil, afterState, nil, audit.StatusSuccess, "")

	// Build response
	resp := createKeyResponse{
		ID:        uuidToString(apiKey.ID),
		Name:      apiKey.Name,
		Key:       key, // Only shown once!
		Role:      string(apiKey.Role),
		CreatedAt: apiKey.CreatedAt.Time.Format(time.RFC3339),
	}
	if apiKey.ExpiresAt.Valid {
		expiresAtStr := apiKey.ExpiresAt.Time.Format(time.RFC3339)
		resp.ExpiresAt = &expiresAtStr
	}

	writeJSON(w, http.StatusOK, resp)
}

// handleListAPIKeys lists all API keys (admin+)
func (s *Server) handleListAPIKeys(w http.ResponseWriter, r *http.Request) {
	pgStore, ok := s.store.(PostgresStoreInterface)
	if !ok {
		InternalError(w, r, "Database store not available")
		return
	}

	keys, err := pgStore.ListAPIKeys(r.Context())
	if err != nil {
		InternalError(w, r, "Failed to list keys")
		return
	}

	// Build response (without revealing actual keys)
	resp := listKeysResponse{
		Keys: make([]keyInfo, 0, len(keys)),
	}

	for _, key := range keys {
		info := keyInfo{
			ID:        uuidToString(key.ID),
			Name:      key.Name,
			Role:      string(key.Role),
			Enabled:   key.Enabled,
			CreatedAt: key.CreatedAt.Time.Format(time.RFC3339),
		}
		if key.LastUsedAt.Valid {
			lastUsedStr := key.LastUsedAt.Time.Format(time.RFC3339)
			info.LastUsedAt = &lastUsedStr
		}
		if key.ExpiresAt.Valid {
			expiresAtStr := key.ExpiresAt.Time.Format(time.RFC3339)
			info.ExpiresAt = &expiresAtStr
		}
		resp.Keys = append(resp.Keys, info)
	}

	writeJSON(w, http.StatusOK, resp)
}

// handleRevokeAPIKey revokes an API key (superadmin only)
func (s *Server) handleRevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	keyID := chi.URLParam(r, "id")
	if keyID == "" {
		BadRequestErrorWithFields(w, r, ErrCodeMissingField, "Missing required parameter", map[string]string{
			"id": "Key ID is required",
		})
		return
	}

	uuid, err := parseUUID(keyID)
	if err != nil {
		BadRequestErrorWithFields(w, r, ErrCodeValidation, "Invalid key ID format", map[string]string{
			"id": "Key ID must be a valid UUID format",
		})
		return
	}

	pgStore, ok := s.store.(PostgresStoreInterface)
	if !ok {
		InternalError(w, r, "Database store not available")
		return
	}

	// Capture before state for audit
	var beforeState map[string]any
	if apiKey, err := pgStore.GetAPIKeyByID(r.Context(), uuid); err == nil {
		beforeState = map[string]any{
			"id":      uuidToString(apiKey.ID),
			"name":    apiKey.Name,
			"role":    string(apiKey.Role),
			"enabled": apiKey.Enabled,
		}
	}

	if err := pgStore.RevokeAPIKey(r.Context(), uuid); err != nil {
		// Log failed audit event
		s.auditLog(r, audit.ActionDeleted, audit.ResourceTypeAPIKey, keyID, "", beforeState, nil, nil, audit.StatusFailure, "Failed to revoke key")
		InternalError(w, r, "Failed to revoke key")
		return
	}

	// After state: key is disabled
	afterState := beforeState
	if afterState != nil {
		afterState["enabled"] = false
	}

	// Log the action (using audit.ActionDeleted for revocation)
	s.auditLog(r, audit.ActionDeleted, audit.ResourceTypeAPIKey, keyID, "", beforeState, afterState, nil, audit.StatusSuccess, "")

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"message": "API key revoked successfully",
	})
}

// --- Audit Log Endpoints ---

type listAuditLogsResponse struct {
	Logs       []auditLogInfo `json:"logs"`
	TotalCount int64          `json:"total_count"`
	Limit      int32          `json:"limit"`
	Offset     int32          `json:"offset"`
}

type auditLogInfo struct {
	ID        string                 `json:"id"`
	Timestamp string                 `json:"timestamp"`
	APIKeyID  *string                `json:"api_key_id,omitempty"`
	Action    string                 `json:"action"`
	Resource  string                 `json:"resource"`
	IPAddress string                 `json:"ip_address"`
	UserAgent string                 `json:"user_agent"`
	Status    int32                  `json:"status"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// handleListAuditLogs lists audit logs with pagination (admin+)
func (s *Server) handleListAuditLogs(w http.ResponseWriter, r *http.Request) {
	// Parse pagination parameters
	limit := int32(50) // default
	offset := int32(0)

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		var l int
		if _, err := fmt.Sscanf(limitStr, "%d", &l); err == nil && l > 0 && l <= 100 {
			limit = int32(l)
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		var o int
		if _, err := fmt.Sscanf(offsetStr, "%d", &o); err == nil && o >= 0 {
			offset = int32(o)
		}
	}

	pgStore, ok := s.store.(PostgresStoreInterface)
	if !ok {
		InternalError(w, r, "Database store not available")
		return
	}

	// Create filter params (no filtering for now, just pagination)
	listParams := dbgen.ListAuditLogsParams{
		Limit:  limit,
		Offset: offset,
		// All filter fields default to NULL (no filtering)
	}

	logs, err := pgStore.ListAuditLogs(r.Context(), listParams)
	if err != nil {
		InternalError(w, r, "Failed to list audit logs")
		return
	}

	countParams := dbgen.CountAuditLogsParams{
		// All filter fields default to NULL (no filtering)
	}

	totalCount, err := pgStore.CountAuditLogs(r.Context(), countParams)
	if err != nil {
		InternalError(w, r, "Failed to count audit logs")
		return
	}

	// Build response
	resp := listAuditLogsResponse{
		Logs:       make([]auditLogInfo, 0, len(logs)),
		TotalCount: totalCount,
		Limit:      limit,
		Offset:     offset,
	}

	for _, log := range logs {
		info := auditLogInfo{
			ID:        uuidToString(log.ID),
			Timestamp: log.Timestamp.Time.Format(time.RFC3339),
			Action:    log.Action,
			Resource:  "", // Legacy field - use resource_type/resource_id instead
			IPAddress: log.IpAddress,
			UserAgent: log.UserAgent,
			Status:    log.Status,
		}
		
		// Set resource from new fields if available
		if log.ResourceType.Valid && log.ResourceID.Valid {
			info.Resource = log.ResourceType.String + "/" + log.ResourceID.String
		} else if log.Resource.Valid {
			info.Resource = log.Resource.String
		}
		
		if log.ApiKeyID.Valid {
			apiKeyIDStr := uuidToString(log.ApiKeyID)
			info.APIKeyID = &apiKeyIDStr
		}
		if len(log.Details) > 0 {
			var details map[string]interface{}
			if err := json.Unmarshal(log.Details, &details); err == nil {
				info.Details = details
			}
		}
		resp.Logs = append(resp.Logs, info)
	}

	writeJSON(w, http.StatusOK, resp)
}

// --- Helper functions ---

// PostgresStoreInterface extends store.Store with postgres-specific methods
type PostgresStoreInterface interface {
	store.Store
	ListAPIKeys(ctx context.Context) ([]dbgen.ApiKey, error)
	CreateAPIKey(ctx context.Context, params dbgen.CreateAPIKeyParams) (dbgen.ApiKey, error)
	GetAPIKeyByID(ctx context.Context, id pgtype.UUID) (dbgen.ApiKey, error)
	RevokeAPIKey(ctx context.Context, id pgtype.UUID) error
	ListAuditLogs(ctx context.Context, params dbgen.ListAuditLogsParams) ([]dbgen.AuditLog, error)
	CountAuditLogs(ctx context.Context, params dbgen.CountAuditLogsParams) (int64, error)
	CreateAuditLog(ctx context.Context, params dbgen.CreateAuditLogParams) error
}

func uuidToString(uuid pgtype.UUID) string {
	if !uuid.Valid {
		return ""
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		uuid.Bytes[0:4], uuid.Bytes[4:6], uuid.Bytes[6:8], uuid.Bytes[8:10], uuid.Bytes[10:16])
}

func parseUUID(s string) (pgtype.UUID, error) {
	var uuid pgtype.UUID
	var b [16]byte
	var parts [5]uint64

	_, err := fmt.Sscanf(s, "%8x-%4x-%4x-%4x-%12x",
		&parts[0], &parts[1], &parts[2], &parts[3], &parts[4])
	if err != nil {
		return uuid, err
	}

	// Convert parts to bytes
	b[0] = byte(parts[0] >> 24)
	b[1] = byte(parts[0] >> 16)
	b[2] = byte(parts[0] >> 8)
	b[3] = byte(parts[0])
	
	b[4] = byte(parts[1] >> 8)
	b[5] = byte(parts[1])
	
	b[6] = byte(parts[2] >> 8)
	b[7] = byte(parts[2])
	
	b[8] = byte(parts[3] >> 8)
	b[9] = byte(parts[3])
	
	b[10] = byte(parts[4] >> 40)
	b[11] = byte(parts[4] >> 32)
	b[12] = byte(parts[4] >> 24)
	b[13] = byte(parts[4] >> 16)
	b[14] = byte(parts[4] >> 8)
	b[15] = byte(parts[4])

	uuid.Bytes = b
	uuid.Valid = true
	return uuid, nil
}

// Helper functions moved to server.go
