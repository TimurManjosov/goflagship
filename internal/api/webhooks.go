package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	dbgen "github.com/TimurManjosov/goflagship/internal/db/gen"
	"github.com/TimurManjosov/goflagship/internal/webhook"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// CreateWebhookRequest represents the request body for creating a webhook
type CreateWebhookRequest struct {
	URL            string   `json:"url"`
	Description    string   `json:"description,omitempty"`
	Events         []string `json:"events"`
	ProjectID      *string  `json:"project_id,omitempty"`
	Environments   []string `json:"environments,omitempty"`
	MaxRetries     int32    `json:"max_retries,omitempty"`
	TimeoutSeconds int32    `json:"timeout_seconds,omitempty"`
}

// UpdateWebhookRequest represents the request body for updating a webhook
type UpdateWebhookRequest struct {
	URL            string   `json:"url"`
	Description    string   `json:"description,omitempty"`
	Enabled        bool     `json:"enabled"`
	Events         []string `json:"events"`
	ProjectID      *string  `json:"project_id,omitempty"`
	Environments   []string `json:"environments,omitempty"`
	MaxRetries     int32    `json:"max_retries,omitempty"`
	TimeoutSeconds int32    `json:"timeout_seconds,omitempty"`
}

// WebhookResponse represents the response for a webhook
type WebhookResponse struct {
	ID              string    `json:"id"`
	URL             string    `json:"url"`
	Description     string    `json:"description,omitempty"`
	Enabled         bool      `json:"enabled"`
	Events          []string  `json:"events"`
	ProjectID       string    `json:"project_id,omitempty"`
	Environments    []string  `json:"environments,omitempty"`
	Secret          string    `json:"secret"`
	MaxRetries      int32     `json:"max_retries"`
	TimeoutSeconds  int32     `json:"timeout_seconds"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	LastTriggeredAt *time.Time `json:"last_triggered_at,omitempty"`
}

// WebhookDeliveryResponse represents a webhook delivery record
type WebhookDeliveryResponse struct {
	ID           string    `json:"id"`
	EventType    string    `json:"event_type"`
	Timestamp    time.Time `json:"timestamp"`
	StatusCode   int       `json:"status_code,omitempty"`
	DurationMs   int       `json:"duration_ms,omitempty"`
	Success      bool      `json:"success"`
	RetryCount   int32     `json:"retry_count"`
	ErrorMessage string    `json:"error_message,omitempty"`
}

// PaginatedDeliveriesResponse represents paginated webhook deliveries
type PaginatedDeliveriesResponse struct {
	Deliveries []WebhookDeliveryResponse `json:"deliveries"`
	Pagination PaginationInfo            `json:"pagination"`
}

// PaginationInfo contains pagination metadata
type PaginationInfo struct {
	Page  int   `json:"page"`
	Limit int   `json:"limit"`
	Total int64 `json:"total"`
}

// handleCreateWebhook creates a new webhook
func (s *Server) handleCreateWebhook(w http.ResponseWriter, r *http.Request) {
	var req CreateWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		BadRequestError(w, r, ErrCodeInvalidJSON, "Invalid JSON: "+err.Error())
		return
	}

	// Validate required fields
	errors := make(map[string]string)
	if req.URL == "" {
		errors["url"] = "URL is required"
	}
	if len(req.Events) == 0 {
		errors["events"] = "At least one event type is required"
	}
	if len(errors) > 0 {
		ValidationError(w, r, "Validation failed", errors)
		return
	}

	// Set defaults
	if req.MaxRetries == 0 {
		req.MaxRetries = 3
	}
	if req.TimeoutSeconds == 0 {
		req.TimeoutSeconds = 10
	}

	// Generate webhook secret
	secret, err := webhook.GenerateSecret()
	if err != nil {
		InternalError(w, r, "Failed to generate webhook secret")
		return
	}

	// Get queries from store
	queries := s.requireQueries(w, r)
	if queries == nil {
		return // Error already written to response
	}

	// Prepare parameters
	params := dbgen.CreateWebhookParams{
		Url:            req.URL,
		Enabled:        true,
		Events:         req.Events,
		Secret:         secret,
		MaxRetries:     req.MaxRetries,
		TimeoutSeconds: req.TimeoutSeconds,
	}

	if req.Description != "" {
		params.Description = pgtype.Text{String: req.Description, Valid: true}
	}

	if req.ProjectID != nil && *req.ProjectID != "" {
		// Parse project ID as UUID
		var projectUUID pgtype.UUID
		if err := projectUUID.Scan(*req.ProjectID); err != nil {
			BadRequestError(w, r, ErrCodeInvalidJSON, "Invalid project_id format")
			return
		}
		params.ProjectID = projectUUID
	}

	if len(req.Environments) > 0 {
		params.Environments = req.Environments
	}

	// Create webhook
	wh, err := queries.CreateWebhook(r.Context(), params)
	if err != nil {
		InternalError(w, r, "Failed to create webhook")
		return
	}

	writeJSON(w, http.StatusCreated, webhookToResponse(wh))
}

// handleListWebhooks lists all webhooks
func (s *Server) handleListWebhooks(w http.ResponseWriter, r *http.Request) {
	queries := s.requireQueries(w, r)
	if queries == nil {
		return // Error already written to response
	}

	webhooks, err := queries.ListWebhooks(r.Context())
	if err != nil {
		InternalError(w, r, "Failed to list webhooks")
		return
	}

	response := make([]WebhookResponse, len(webhooks))
	for i, wh := range webhooks {
		response[i] = webhookToResponse(wh)
	}

	writeJSON(w, http.StatusOK, response)
}

// handleGetWebhook gets a specific webhook by ID
func (s *Server) handleGetWebhook(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		BadRequestError(w, r, ErrCodeInvalidJSON, "Webhook ID is required")
		return
	}

	var webhookID pgtype.UUID
	if err := webhookID.Scan(idStr); err != nil {
		BadRequestError(w, r, ErrCodeInvalidJSON, "Invalid webhook ID format")
		return
	}

	queries := s.requireQueries(w, r)
	if queries == nil {
		return // Error already written to response
	}

	wh, err := queries.GetWebhook(r.Context(), webhookID)
	if err != nil {
		NotFoundError(w, r, "Webhook not found")
		return
	}

	writeJSON(w, http.StatusOK, webhookToResponse(wh))
}

// handleUpdateWebhook updates a webhook
func (s *Server) handleUpdateWebhook(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		BadRequestError(w, r, ErrCodeInvalidJSON, "Webhook ID is required")
		return
	}

	var webhookID pgtype.UUID
	if err := webhookID.Scan(idStr); err != nil {
		BadRequestError(w, r, ErrCodeInvalidJSON, "Invalid webhook ID format")
		return
	}

	var req UpdateWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		BadRequestError(w, r, ErrCodeInvalidJSON, "Invalid JSON: "+err.Error())
		return
	}

	// Validate required fields
	errors := make(map[string]string)
	if req.URL == "" {
		errors["url"] = "URL is required"
	}
	if len(req.Events) == 0 {
		errors["events"] = "At least one event type is required"
	}
	if len(errors) > 0 {
		ValidationError(w, r, "Validation failed", errors)
		return
	}

	queries := s.requireQueries(w, r)
	if queries == nil {
		return // Error already written to response
	}

	// Prepare parameters
	params := dbgen.UpdateWebhookParams{
		ID:             webhookID,
		Url:            req.URL,
		Enabled:        req.Enabled,
		Events:         req.Events,
		MaxRetries:     req.MaxRetries,
		TimeoutSeconds: req.TimeoutSeconds,
	}

	if req.Description != "" {
		params.Description = pgtype.Text{String: req.Description, Valid: true}
	}

	if req.ProjectID != nil && *req.ProjectID != "" {
		var projectUUID pgtype.UUID
		if err := projectUUID.Scan(*req.ProjectID); err != nil {
			BadRequestError(w, r, ErrCodeInvalidJSON, "Invalid project_id format")
			return
		}
		params.ProjectID = projectUUID
	}

	if len(req.Environments) > 0 {
		params.Environments = req.Environments
	}

	// Update webhook
	if err := queries.UpdateWebhook(r.Context(), params); err != nil {
		InternalError(w, r, "Failed to update webhook")
		return
	}

	// Fetch updated webhook to return
	wh, err := queries.GetWebhook(r.Context(), webhookID)
	if err != nil {
		InternalError(w, r, "Failed to fetch updated webhook")
		return
	}

	writeJSON(w, http.StatusOK, webhookToResponse(wh))
}

// handleDeleteWebhook deletes a webhook
func (s *Server) handleDeleteWebhook(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		BadRequestError(w, r, ErrCodeInvalidJSON, "Webhook ID is required")
		return
	}

	var webhookID pgtype.UUID
	if err := webhookID.Scan(idStr); err != nil {
		BadRequestError(w, r, ErrCodeInvalidJSON, "Invalid webhook ID format")
		return
	}

	queries := s.requireQueries(w, r)
	if queries == nil {
		return // Error already written to response
	}

	if err := queries.DeleteWebhook(r.Context(), webhookID); err != nil {
		InternalError(w, r, "Failed to delete webhook")
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleListWebhookDeliveries lists webhook delivery attempts
func (s *Server) handleListWebhookDeliveries(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		BadRequestError(w, r, ErrCodeInvalidJSON, "Webhook ID is required")
		return
	}

	var webhookID pgtype.UUID
	if err := webhookID.Scan(idStr); err != nil {
		BadRequestError(w, r, ErrCodeInvalidJSON, "Invalid webhook ID format")
		return
	}

	// Parse pagination parameters
	page := 1
	limit := 20
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	offset := (page - 1) * limit

	queries := s.requireQueries(w, r)
	if queries == nil {
		return // Error already written to response
	}

	// Get deliveries
	deliveries, err := queries.ListWebhookDeliveries(r.Context(), dbgen.ListWebhookDeliveriesParams{
		WebhookID: webhookID,
		Limit:     int32(limit),
		Offset:    int32(offset),
	})
	if err != nil {
		InternalError(w, r, "Failed to list deliveries")
		return
	}

	// Get total count
	total, err := queries.CountWebhookDeliveries(r.Context(), webhookID)
	if err != nil {
		InternalError(w, r, "Failed to count deliveries")
		return
	}

	response := PaginatedDeliveriesResponse{
		Deliveries: make([]WebhookDeliveryResponse, len(deliveries)),
		Pagination: PaginationInfo{
			Page:  page,
			Limit: limit,
			Total: total,
		},
	}

	for i, d := range deliveries {
		response.Deliveries[i] = deliveryToResponse(d)
	}

	writeJSON(w, http.StatusOK, response)
}

// handleTestWebhook manually triggers a test webhook
func (s *Server) handleTestWebhook(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		BadRequestError(w, r, ErrCodeInvalidJSON, "Webhook ID is required")
		return
	}

	var webhookID pgtype.UUID
	if err := webhookID.Scan(idStr); err != nil {
		BadRequestError(w, r, ErrCodeInvalidJSON, "Invalid webhook ID format")
		return
	}

	// Get webhook to verify it exists
	queries := s.requireQueries(w, r)
	if queries == nil {
		return // Error already written to response
	}

	wh, err := queries.GetWebhook(r.Context(), webhookID)
	if err != nil {
		NotFoundError(w, r, "Webhook not found")
		return
	}

	// Dispatch a test event
	if s.webhookDispatcher != nil {
		testEvent := webhook.Event{
			Type:        "webhook.test",
			Timestamp:   time.Now(),
			Environment: "test",
			Resource: webhook.Resource{
				Type: "webhook",
				Key:  formatUUID(wh.ID),
			},
			Data: webhook.EventData{
				After: map[string]any{
					"message": "This is a test webhook delivery",
				},
			},
			Metadata: webhook.Metadata{
				RequestID: r.Header.Get("X-Request-ID"),
			},
		}

		s.webhookDispatcher.Dispatch(testEvent)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"message": "Test webhook dispatched",
	})
}

// webhookToResponse converts a dbgen.Webhook to a WebhookResponse
func webhookToResponse(wh dbgen.Webhook) WebhookResponse {
	resp := WebhookResponse{
		ID:             formatUUID(wh.ID),
		URL:            wh.Url,
		Enabled:        wh.Enabled,
		Events:         wh.Events,
		Secret:         wh.Secret,
		MaxRetries:     wh.MaxRetries,
		TimeoutSeconds: wh.TimeoutSeconds,
		CreatedAt:      wh.CreatedAt.Time,
		UpdatedAt:      wh.UpdatedAt.Time,
	}

	if wh.Description.Valid {
		resp.Description = wh.Description.String
	}

	if wh.ProjectID.Valid {
		resp.ProjectID = formatUUID(wh.ProjectID)
	}

	if len(wh.Environments) > 0 {
		resp.Environments = wh.Environments
	}

	if wh.LastTriggeredAt.Valid {
		t := wh.LastTriggeredAt.Time
		resp.LastTriggeredAt = &t
	}

	return resp
}

// deliveryToResponse converts a dbgen.WebhookDelivery to a WebhookDeliveryResponse
func deliveryToResponse(d dbgen.WebhookDelivery) WebhookDeliveryResponse {
	resp := WebhookDeliveryResponse{
		ID:         formatUUID(d.ID),
		EventType:  d.EventType,
		Timestamp:  d.Timestamp.Time,
		Success:    d.Success,
		RetryCount: d.RetryCount,
	}

	if d.StatusCode.Valid {
		resp.StatusCode = int(d.StatusCode.Int32)
	}

	if d.DurationMs.Valid {
		resp.DurationMs = int(d.DurationMs.Int32)
	}

	if d.ErrorMessage.Valid {
		resp.ErrorMessage = d.ErrorMessage.String
	}

	return resp
}
