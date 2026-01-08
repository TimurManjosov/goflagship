package api

import (
	"context"
	"encoding/csv"
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
		"id":         uuidToString(apiKey.ID),
		"name":       apiKey.Name,
		"role":       string(apiKey.Role),
		"enabled":    apiKey.Enabled,
		"created_at": formatTimestamp(apiKey.CreatedAt),
	}
	if apiKey.ExpiresAt.Valid {
		afterState["expires_at"] = formatTimestamp(apiKey.ExpiresAt)
	}
	s.auditLog(r, audit.ActionCreated, audit.ResourceTypeAPIKey, uuidToString(apiKey.ID), "", nil, afterState, nil, audit.StatusSuccess, "")

	// Build response
	resp := createKeyResponse{
		ID:        uuidToString(apiKey.ID),
		Name:      apiKey.Name,
		Key:       key, // Only shown once!
		Role:      string(apiKey.Role),
		CreatedAt: formatTimestamp(apiKey.CreatedAt),
		ExpiresAt: formatOptionalTimestamp(apiKey.ExpiresAt),
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
			ID:         uuidToString(key.ID),
			Name:       key.Name,
			Role:       string(key.Role),
			Enabled:    key.Enabled,
			CreatedAt:  formatTimestamp(key.CreatedAt),
			LastUsedAt: formatOptionalTimestamp(key.LastUsedAt),
			ExpiresAt:  formatOptionalTimestamp(key.ExpiresAt),
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

	// After state: key is disabled (create proper copy)
	var afterState map[string]any
	if beforeState != nil {
		afterState = make(map[string]any)
		for k, v := range beforeState {
			afterState[k] = v
		}
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
	Logs       []auditLogInfo   `json:"logs"`
	Pagination paginationInfo   `json:"pagination"`
}

type paginationInfo struct {
	Page  int32 `json:"page"`
	Limit int32 `json:"limit"`
	Total int64 `json:"total"`
	Pages int32 `json:"pages"`
}

type auditLogInfo struct {
	ID           string                 `json:"id"`
	Timestamp    string                 `json:"timestamp"`
	Action       string                 `json:"action"`
	ResourceType string                 `json:"resource_type,omitempty"`
	ResourceID   string                 `json:"resource_id,omitempty"`
	ProjectID    string                 `json:"project_id,omitempty"`
	Environment  string                 `json:"environment,omitempty"`
	BeforeState  map[string]interface{} `json:"before_state,omitempty"`
	AfterState   map[string]interface{} `json:"after_state,omitempty"`
	Changes      map[string]interface{} `json:"changes,omitempty"`
	IPAddress    string                 `json:"ip_address"`
	UserAgent    string                 `json:"user_agent"`
	RequestID    string                 `json:"request_id,omitempty"`
	APIKeyID     *string                `json:"api_key_id,omitempty"`
	UserEmail    string                 `json:"user_email,omitempty"`
	Status       int32                  `json:"status"`
	ErrorMessage string                 `json:"error_message,omitempty"`
	Resource     string                 `json:"resource,omitempty"` // Legacy field
}

// handleListAuditLogs lists audit logs with pagination and filtering (admin+)
func (s *Server) handleListAuditLogs(w http.ResponseWriter, r *http.Request) {
	// Parse pagination parameters
	page := int32(1) // default page
	limit := int32(20) // default limit per spec
	
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		var p int
		if _, err := fmt.Sscanf(pageStr, "%d", &p); err == nil && p > 0 {
			page = int32(p)
		}
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		var l int
		if _, err := fmt.Sscanf(limitStr, "%d", &l); err == nil && l > 0 && l <= 100 {
			limit = int32(l)
		}
	}
	
	// Calculate offset from page
	offset := (page - 1) * limit

	// Parse filter parameters
	var projectID, resourceType, resourceID, action pgtype.Text
	var startDate, endDate pgtype.Timestamptz
	
	if p := r.URL.Query().Get("projectId"); p != "" {
		projectID = pgtype.Text{String: p, Valid: true}
	}
	
	if rt := r.URL.Query().Get("resourceType"); rt != "" {
		resourceType = pgtype.Text{String: rt, Valid: true}
	}
	
	if rid := r.URL.Query().Get("resourceId"); rid != "" {
		resourceID = pgtype.Text{String: rid, Valid: true}
	}
	
	if a := r.URL.Query().Get("action"); a != "" {
		action = pgtype.Text{String: a, Valid: true}
	}
	
	if sd := r.URL.Query().Get("startDate"); sd != "" {
		if t, err := time.Parse(time.RFC3339, sd); err == nil {
			startDate = pgtype.Timestamptz{Time: t, Valid: true}
		}
	}
	
	if ed := r.URL.Query().Get("endDate"); ed != "" {
		if t, err := time.Parse(time.RFC3339, ed); err == nil {
			endDate = pgtype.Timestamptz{Time: t, Valid: true}
		}
	}

	pgStore, ok := s.store.(PostgresStoreInterface)
	if !ok {
		InternalError(w, r, "Database store not available")
		return
	}

	// Create filter params with all query parameters
	listParams := dbgen.ListAuditLogsParams{
		Limit:        limit,
		Offset:       offset,
		ProjectID:    projectID,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Action:       action,
		StartDate:    startDate,
		EndDate:      endDate,
	}

	logs, err := pgStore.ListAuditLogs(r.Context(), listParams)
	if err != nil {
		InternalError(w, r, "Failed to list audit logs")
		return
	}

	countParams := dbgen.CountAuditLogsParams{
		ProjectID:    projectID,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Action:       action,
		StartDate:    startDate,
		EndDate:      endDate,
	}

	totalCount, err := pgStore.CountAuditLogs(r.Context(), countParams)
	if err != nil {
		InternalError(w, r, "Failed to count audit logs")
		return
	}

	// Calculate total pages
	pages := int32((totalCount + int64(limit) - 1) / int64(limit))
	if pages == 0 {
		pages = 1
	}

	// Build response
	resp := listAuditLogsResponse{
		Logs: make([]auditLogInfo, 0, len(logs)),
		Pagination: paginationInfo{
			Page:  page,
			Limit: limit,
			Total: totalCount,
			Pages: pages,
		},
	}

	for _, log := range logs {
		info := auditLogInfo{
			ID:        uuidToString(log.ID),
			Timestamp: formatTimestamp(log.Timestamp),
			Action:    log.Action,
			IPAddress: log.IpAddress,
			UserAgent: log.UserAgent,
			Status:    log.Status,
		}
		
		// Set new fields
		if log.ResourceType.Valid {
			info.ResourceType = log.ResourceType.String
		}
		
		if log.ResourceID.Valid {
			info.ResourceID = log.ResourceID.String
		}
		
		if log.ProjectID.Valid {
			info.ProjectID = log.ProjectID.String
		}
		
		if log.Environment.Valid {
			info.Environment = log.Environment.String
		}
		
		if log.RequestID.Valid {
			info.RequestID = log.RequestID.String
		}
		
		if log.UserEmail.Valid {
			info.UserEmail = log.UserEmail.String
		}
		
		if log.ErrorMessage.Valid {
			info.ErrorMessage = log.ErrorMessage.String
		}
		
		// Set legacy resource field for backward compatibility
		if log.ResourceType.Valid && log.ResourceID.Valid {
			info.Resource = log.ResourceType.String + "/" + log.ResourceID.String
		} else if log.Resource.Valid {
			info.Resource = log.Resource.String
		}
		
		if log.ApiKeyID.Valid {
			apiKeyIDStr := uuidToString(log.ApiKeyID)
			info.APIKeyID = &apiKeyIDStr
		}
		
		// Parse JSONB fields
		if len(log.BeforeState) > 0 {
			var beforeState map[string]interface{}
			if err := json.Unmarshal(log.BeforeState, &beforeState); err == nil {
				info.BeforeState = beforeState
			}
		}
		
		if len(log.AfterState) > 0 {
			var afterState map[string]interface{}
			if err := json.Unmarshal(log.AfterState, &afterState); err == nil {
				info.AfterState = afterState
			}
		}
		
		if len(log.Changes) > 0 {
			var changes map[string]interface{}
			if err := json.Unmarshal(log.Changes, &changes); err == nil {
				info.Changes = changes
			}
		}
		resp.Logs = append(resp.Logs, info)
	}

	writeJSON(w, http.StatusOK, resp)
}

// handleExportAuditLogs exports audit logs in various formats (admin+)
func (s *Server) handleExportAuditLogs(w http.ResponseWriter, r *http.Request) {
	// Parse format parameter (required)
	format := r.URL.Query().Get("format")
	if format == "" {
		BadRequestErrorWithFields(w, r, ErrCodeMissingField, "Missing required parameter", map[string]string{
			"format": "Format parameter is required (csv, json, or jsonl)",
		})
		return
	}
	
	if format != "csv" && format != "json" && format != "jsonl" {
		BadRequestErrorWithFields(w, r, ErrCodeValidation, "Invalid format", map[string]string{
			"format": "Format must be csv, json, or jsonl",
		})
		return
	}

	// Parse filter parameters (same as list endpoint)
	var projectID, resourceType, resourceID, action pgtype.Text
	var startDate, endDate pgtype.Timestamptz
	
	if p := r.URL.Query().Get("projectId"); p != "" {
		projectID = pgtype.Text{String: p, Valid: true}
	}
	
	if rt := r.URL.Query().Get("resourceType"); rt != "" {
		resourceType = pgtype.Text{String: rt, Valid: true}
	}
	
	if rid := r.URL.Query().Get("resourceId"); rid != "" {
		resourceID = pgtype.Text{String: rid, Valid: true}
	}
	
	if a := r.URL.Query().Get("action"); a != "" {
		action = pgtype.Text{String: a, Valid: true}
	}
	
	if sd := r.URL.Query().Get("startDate"); sd != "" {
		if t, err := time.Parse(time.RFC3339, sd); err == nil {
			startDate = pgtype.Timestamptz{Time: t, Valid: true}
		}
	}
	
	if ed := r.URL.Query().Get("endDate"); ed != "" {
		if t, err := time.Parse(time.RFC3339, ed); err == nil {
			endDate = pgtype.Timestamptz{Time: t, Valid: true}
		}
	}

	pgStore, ok := s.store.(PostgresStoreInterface)
	if !ok {
		InternalError(w, r, "Database store not available")
		return
	}

	// Fetch all matching logs (no pagination for export)
	listParams := dbgen.ListAuditLogsParams{
		Limit:        maxAuditExportLimit,
		Offset:       0,
		ProjectID:    projectID,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Action:       action,
		StartDate:    startDate,
		EndDate:      endDate,
	}

	logs, err := pgStore.ListAuditLogs(r.Context(), listParams)
	if err != nil {
		InternalError(w, r, "Failed to list audit logs")
		return
	}

	// Convert to auditLogInfo for consistent formatting
	auditLogs := make([]auditLogInfo, 0, len(logs))
	for _, log := range logs {
		info := auditLogInfo{
			ID:        uuidToString(log.ID),
			Timestamp: formatTimestamp(log.Timestamp),
			Action:    log.Action,
			IPAddress: log.IpAddress,
			UserAgent: log.UserAgent,
			Status:    log.Status,
		}
		
		if log.ResourceType.Valid {
			info.ResourceType = log.ResourceType.String
		}
		
		if log.ResourceID.Valid {
			info.ResourceID = log.ResourceID.String
		}
		
		if log.ProjectID.Valid {
			info.ProjectID = log.ProjectID.String
		}
		
		if log.Environment.Valid {
			info.Environment = log.Environment.String
		}
		
		if log.RequestID.Valid {
			info.RequestID = log.RequestID.String
		}
		
		if log.UserEmail.Valid {
			info.UserEmail = log.UserEmail.String
		}
		
		if log.ErrorMessage.Valid {
			info.ErrorMessage = log.ErrorMessage.String
		}
		
		if log.ApiKeyID.Valid {
			apiKeyIDStr := uuidToString(log.ApiKeyID)
			info.APIKeyID = &apiKeyIDStr
		}
		
		// Don't parse JSONB fields for CSV (too complex), but include for JSON
		if format != "csv" {
			if len(log.BeforeState) > 0 {
				var beforeState map[string]interface{}
				if err := json.Unmarshal(log.BeforeState, &beforeState); err == nil {
					info.BeforeState = beforeState
				}
			}
			
			if len(log.AfterState) > 0 {
				var afterState map[string]interface{}
				if err := json.Unmarshal(log.AfterState, &afterState); err == nil {
					info.AfterState = afterState
				}
			}
			
			if len(log.Changes) > 0 {
				var changes map[string]interface{}
				if err := json.Unmarshal(log.Changes, &changes); err == nil {
					info.Changes = changes
				}
			}
		}
		
		auditLogs = append(auditLogs, info)
	}

	// Export based on format
	switch format {
	case "csv":
		exportCSV(w, auditLogs)
	case "json":
		exportJSON(w, auditLogs)
	case "jsonl":
		exportJSONL(w, auditLogs)
	}
}

// exportCSV exports audit logs as CSV using proper CSV encoding
func exportCSV(w http.ResponseWriter, logs []auditLogInfo) {
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=audit-logs.csv")
	
	csvWriter := csv.NewWriter(w)
	defer csvWriter.Flush()
	
	// Write CSV header
	if err := csvWriter.Write([]string{
		"ID", "Timestamp", "Action", "ResourceType", "ResourceID", 
		"ProjectID", "Environment", "IPAddress", "UserAgent", "RequestID", 
		"APIKeyID", "UserEmail", "Status", "ErrorMessage",
	}); err != nil {
		// Header already sent, can't return error response - log and return
		return
	}
	
	// Write CSV rows
	for _, log := range logs {
		apiKeyID := ""
		if log.APIKeyID != nil {
			apiKeyID = *log.APIKeyID
		}
		
		if err := csvWriter.Write([]string{
			log.ID,
			log.Timestamp,
			log.Action,
			log.ResourceType,
			log.ResourceID,
			log.ProjectID,
			log.Environment,
			log.IPAddress,
			log.UserAgent,
			log.RequestID,
			apiKeyID,
			log.UserEmail,
			fmt.Sprintf("%d", log.Status),
			log.ErrorMessage,
		}); err != nil {
			// Can't return error at this point, just stop writing
			return
		}
	}
}

// exportJSON exports audit logs as JSON array
func exportJSON(w http.ResponseWriter, logs []auditLogInfo) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=audit-logs.json")
	
	json.NewEncoder(w).Encode(logs)
}

// exportJSONL exports audit logs as JSON Lines (one JSON object per line)
func exportJSONL(w http.ResponseWriter, logs []auditLogInfo) {
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Content-Disposition", "attachment; filename=audit-logs.jsonl")
	
	encoder := json.NewEncoder(w)
	for _, log := range logs {
		encoder.Encode(log)
	}
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
