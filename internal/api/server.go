package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/TimurManjosov/goflagship/internal/audit"
	"github.com/TimurManjosov/goflagship/internal/auth"
	dbgen "github.com/TimurManjosov/goflagship/internal/db/gen"
	"github.com/TimurManjosov/goflagship/internal/snapshot"
	"github.com/TimurManjosov/goflagship/internal/store"
	"github.com/TimurManjosov/goflagship/internal/targeting"
	"github.com/TimurManjosov/goflagship/internal/telemetry"
	"github.com/TimurManjosov/goflagship/internal/validation"
	"github.com/TimurManjosov/goflagship/internal/webhook"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"
)

const (
	// auditQueueSize is the size of the buffered channel for audit log entries
	auditQueueSize = 100
	
	// maxAuditExportLimit is the maximum number of audit logs that can be exported at once
	maxAuditExportLimit = 10000
)

type Server struct {
	store             store.Store
	env               string
	adminAPIKey       string
	auth              *auth.Authenticator
	auditService      *audit.Service
	webhookDispatcher *webhook.Dispatcher
}

func NewServer(s store.Store, env, adminKey string) *Server {
	// Create authenticator with key store
	var keyStore auth.KeyStore
	if pgStore, ok := s.(auth.KeyStore); ok {
		keyStore = pgStore
	}

	authenticator := auth.NewAuthenticator(keyStore, adminKey)

	// Create audit service and webhook dispatcher
	var auditSvc *audit.Service
	var webhookDisp *webhook.Dispatcher
	if pgStore, ok := s.(PostgresStoreInterface); ok {
		queries := getQueriesFromStore(pgStore)
		if queries != nil {
			sink := audit.NewPostgresSink(queries)
			auditSvc = audit.NewService(sink, audit.SystemClock{}, audit.UUIDGenerator{}, audit.NewDefaultRedactor(), auditQueueSize)
			
			// Create and start webhook dispatcher
			webhookDisp = webhook.NewDispatcher(queries)
			webhookDisp.Start()
		}
	}

	srv := &Server{
		store:             s,
		env:               env,
		adminAPIKey:       adminKey,
		auth:              authenticator,
		auditService:      auditSvc,
		webhookDispatcher: webhookDisp,
	}

	return srv
}

// Helper to extract *dbgen.Queries from PostgresStoreInterface
func getQueriesFromStore(pgStore PostgresStoreInterface) *dbgen.Queries {
	// This is a workaround - in a real implementation, we'd expose Queries directly
	// For now, we'll use reflection or a type assertion
	type queriesGetter interface {
		GetQueries() *dbgen.Queries
	}
	if qg, ok := pgStore.(queriesGetter); ok {
		return qg.GetQueries()
	}
	// If we can't get queries, audit service will be nil and we'll skip audit logging
	return nil
}

// requirePostgresStore ensures the store implements PostgresStoreInterface and returns it.
// If the store doesn't support PostgreSQL operations, it writes an internal error response and returns nil.
// This is a convenience helper to reduce repeated type assertions and error handling.
//
// Usage:
//   pgStore := s.requirePostgresStore(w, r)
//   if pgStore == nil {
//       return // Error already written to response
//   }
//   // Use pgStore...
func (s *Server) requirePostgresStore(w http.ResponseWriter, r *http.Request) PostgresStoreInterface {
	if pgStore, ok := s.store.(PostgresStoreInterface); ok {
		return pgStore
	}
	InternalError(w, r, "Database store not available")
	return nil
}

// requireQueries extracts database queries from the store.
// If queries are not available, it writes an internal error response and returns nil.
// This is a convenience helper for handlers that need direct database access.
//
// Usage:
//   queries := s.requireQueries(w, r)
//   if queries == nil {
//       return // Error already written to response
//   }
//   // Use queries...
func (s *Server) requireQueries(w http.ResponseWriter, r *http.Request) *dbgen.Queries {
	pgStore := s.requirePostgresStore(w, r)
	if pgStore == nil {
		return nil // Error already written
	}
	queries := getQueriesFromStore(pgStore)
	if queries == nil {
		InternalError(w, r, "Database queries not available")
	}
	return queries
}

func (s *Server) Router() http.Handler {
	// inside (s *Server) Router():
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Recoverer)
	r.Use(telemetry.Middleware)

	// CORS for browser clients (adjust origins as needed)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000", "http://localhost:5173", "http://localhost:8080"},
		AllowedMethods:   []string{"GET", "POST", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "If-None-Match"},
		ExposedHeaders:   []string{"ETag"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	// Normal routes with timeout + rate limit
	r.Group(func(r chi.Router) {
		r.Use(middleware.Timeout(5 * time.Second))
		r.Use(httprate.LimitByIP(100, time.Minute)) // 100 req/min per IP

		r.Get("/healthz", s.handleHealth)
		r.Get("/v1/flags/snapshot", s.handleSnapshot)

		// Evaluate endpoint - public, no auth required by default
		// Higher rate limit for evaluation (300 req/min per IP)
		r.Group(func(r chi.Router) {
			r.Use(httprate.LimitByIP(300, time.Minute))
			r.Post("/v1/flags/evaluate", s.handleEvaluate)
			r.Get("/v1/flags/evaluate", s.handleEvaluateGET)
		})

		r.Route("/v1/flags", func(r chi.Router) {
			r.Use(s.auth.RequireAuth(auth.RoleAdmin))
			r.Post("/", s.handleUpsertFlag)
			r.Delete("/", s.handleDeleteFlag)
		})

		// Admin API key management routes (superadmin only)
		r.Route("/v1/admin/keys", func(r chi.Router) {
			r.Use(s.auth.RequireAuth(auth.RoleSuperadmin))
			r.Post("/", s.handleCreateAPIKey)
			r.Get("/", s.handleListAPIKeys)
			r.Delete("/{id}", s.handleRevokeAPIKey)
		})

		// Webhook management routes (admin+)
		r.Route("/v1/admin/webhooks", func(r chi.Router) {
			r.Use(s.auth.RequireAuth(auth.RoleAdmin))
			r.Get("/", s.handleListWebhooks)
			r.Post("/", s.handleCreateWebhook)
			r.Get("/{id}", s.handleGetWebhook)
			r.Put("/{id}", s.handleUpdateWebhook)
			r.Delete("/{id}", s.handleDeleteWebhook)
			r.Get("/{id}/deliveries", s.handleListWebhookDeliveries)
			r.Post("/{id}/test", s.handleTestWebhook)
		})

		// Audit logs routes (admin+)
		r.With(s.auth.RequireAuth(auth.RoleAdmin)).Get("/v1/admin/audit-logs", s.handleListAuditLogs)
		r.With(s.auth.RequireAuth(auth.RoleAdmin)).Get("/v1/admin/audit-logs/export", s.handleExportAuditLogs)
	})

	// SSE route: no timeout, but optional gentle rate limit on connects
	r.Group(func(r chi.Router) {
		r.Use(httprate.LimitByIP(30, time.Minute)) // 30 connects/min per IP
		r.Get("/v1/flags/stream", s.handleStream)
	})

	// Serve static files from ./sdk directory
	// This allows accessing admin.html and index.html from the API server
	fileServer := http.FileServer(http.Dir("./sdk"))
	r.Handle("/*", fileServer)

	return r
}

func (s *Server) handleSnapshot(w http.ResponseWriter, req *http.Request) {
	snap := snapshot.Load()
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Header().Set("ETag", snap.ETag)

	if inm := req.Header.Get("If-None-Match"); inm != "" && inm == snap.ETag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(snap)
}

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	// Proper headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Check flusher
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Subscribe to updates
	updates, unsubscribe := snapshot.Subscribe()
	defer unsubscribe()

	// Send init immediately
	snap := snapshot.Load()
	writeSSE(w, "init", map[string]string{"etag": snap.ETag})
	flusher.Flush()

	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()

	ctx := r.Context()
	for {
		select {
		case etag, ok := <-updates:
			if !ok {
				return
			}
			writeSSE(w, "update", map[string]string{"etag": etag})
			flusher.Flush()

		case <-ticker.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()

		case <-ctx.Done():
			return
		}
	}
}

func writeSSE(w http.ResponseWriter, event string, data any) {
	dataJSON, err := json.Marshal(data)
	if err != nil {
		// Fallback to error message if marshaling fails
		dataJSON = []byte(`{"error":"marshal failed"}`)
	}
	w.Write([]byte("event: " + event + "\n"))
	w.Write([]byte("data: "))
	w.Write(dataJSON)
	w.Write([]byte("\n\n"))
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// ---- handlers ----

// variantRequest represents a variant in the API request
type variantRequest struct {
	Name   string         `json:"name"`
	Weight int            `json:"weight"`
	Config map[string]any `json:"config,omitempty"`
}

type upsertRequest struct {
	Key         string           `json:"key"`
	Description string           `json:"description"`
	Enabled     bool             `json:"enabled"`
	Rollout     int32            `json:"rollout"`
	Expression  *string          `json:"expression,omitempty"`
	Config      map[string]any   `json:"config,omitempty"`
	Variants    []variantRequest `json:"variants,omitempty"` // For A/B testing
	Env         *string          `json:"env,omitempty"`      // defaults to s.env
}

type upsertResponse struct {
	OK   bool   `json:"ok"`
	ETag string `json:"etag"`
}

func (s *Server) handleUpsertFlag(w http.ResponseWriter, r *http.Request) {
	var req upsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		BadRequestError(w, r, ErrCodeInvalidJSON, "Invalid JSON: "+err.Error())
		return
	}

	// default env
	env := s.env
	if req.Env != nil && strings.TrimSpace(*req.Env) != "" {
		env = strings.TrimSpace(*req.Env)
	}

	// Convert variants for validation
	var variantParams []validation.VariantValidationParams
	for _, v := range req.Variants {
		variantParams = append(variantParams, validation.VariantValidationParams{
			Name:   v.Name,
			Weight: v.Weight,
		})
	}

	// Validate all fields using the validation package
	validationResult := validation.ValidateFlag(validation.FlagValidationParams{
		Key:         req.Key,
		Env:         env,
		Description: req.Description,
		Rollout:     req.Rollout,
		Variants:    variantParams,
	})

	if !validationResult.Valid {
		ValidationError(w, r, "Validation failed for one or more fields", validationResult.Errors)
		return
	}

	// Validate expression if provided (expression validation is separate)
	if req.Expression != nil && *req.Expression != "" {
		if err := targeting.ValidateExpression(*req.Expression); err != nil {
			BadRequestErrorWithFields(w, r, ErrCodeInvalidExpression, "Invalid expression", map[string]string{
				"expression": err.Error(),
			})
			return
		}
	}

	// Convert variants to store type
	var variants []store.Variant
	if len(req.Variants) > 0 {
		variants = make([]store.Variant, len(req.Variants))
		for i, v := range req.Variants {
			variants[i] = store.Variant{
				Name:   v.Name,
				Weight: v.Weight,
				Config: v.Config,
			}
		}
	}

	// Capture before state for audit
	var beforeState map[string]any
	isCreate := false
	if oldFlag, err := s.store.GetFlagByKey(r.Context(), req.Key); err == nil {
		beforeState = flagToMap(oldFlag)
	} else {
		isCreate = true
	}

	// upsert via store
	params := store.UpsertParams{
		Key:         req.Key,
		Description: req.Description,
		Enabled:     req.Enabled,
		Rollout:     req.Rollout,
		Expression:  req.Expression,
		Config:      req.Config,
		Variants:    variants,
		Env:         env,
	}
	if err := s.store.UpsertFlag(r.Context(), params); err != nil {
		// Log failed audit event
		s.auditLog(r, audit.ActionUpdated, audit.ResourceTypeFlag, req.Key, env, nil, nil, nil, audit.StatusFailure, "Failed to save flag")
		InternalError(w, r, "Failed to save flag")
		return
	}

	// Capture after state for audit
	var afterState map[string]any
	if newFlag, err := s.store.GetFlagByKey(r.Context(), req.Key); err == nil {
		afterState = flagToMap(newFlag)
	}

	// rebuild in-memory snapshot (read fresh rows for env)
	if err := s.RebuildSnapshot(r.Context(), env); err != nil {
		InternalError(w, r, "Failed to rebuild snapshot")
		return
	}

	// Log successful audit event
	action := audit.ActionUpdated
	if isCreate {
		action = audit.ActionCreated
	}
	changes := audit.ComputeChanges(beforeState, afterState)
	s.auditLog(r, action, audit.ResourceTypeFlag, req.Key, env, beforeState, afterState, changes, audit.StatusSuccess, "")

	// Dispatch webhook event
	s.dispatchWebhookEvent(r, req.Key, env, beforeState, afterState, changes)

	// respond with new ETag
	writeJSON(w, http.StatusOK, upsertResponse{
		OK:   true,
		ETag: snapshot.Load().ETag,
	})
}

func (s *Server) handleDeleteFlag(w http.ResponseWriter, r *http.Request) {
	// Extract query parameters
	key := strings.TrimSpace(r.URL.Query().Get("key"))
	env := strings.TrimSpace(r.URL.Query().Get("env"))

	// Validate required parameters with field-level errors
	errors := make(map[string]string)
	if key == "" {
		errors["key"] = "Key query parameter is required"
	}
	if env == "" {
		errors["env"] = "Env query parameter is required"
	}
	if len(errors) > 0 {
		ValidationError(w, r, "Missing required parameters", errors)
		return
	}

	// Capture before state for audit
	var beforeState map[string]any
	if oldFlag, err := s.store.GetFlagByKey(r.Context(), key); err == nil {
		beforeState = flagToMap(oldFlag)
	}

	// Delete from store
	if err := s.store.DeleteFlag(r.Context(), key, env); err != nil {
		// Log failed audit event
		s.auditLog(r, audit.ActionDeleted, audit.ResourceTypeFlag, key, env, beforeState, nil, nil, audit.StatusFailure, "Failed to delete flag")
		InternalError(w, r, "Failed to delete flag")
		return
	}

	// Rebuild snapshot
	if err := s.RebuildSnapshot(r.Context(), env); err != nil {
		InternalError(w, r, "Failed to rebuild snapshot")
		return
	}

	// Log successful audit event (after state is nil for delete)
	s.auditLog(r, audit.ActionDeleted, audit.ResourceTypeFlag, key, env, beforeState, nil, nil, audit.StatusSuccess, "")

	// Dispatch webhook event for deletion
	s.dispatchWebhookEvent(r, key, env, beforeState, nil, nil)

	// Respond with new ETag (idempotent: always returns success)
	writeJSON(w, http.StatusOK, upsertResponse{
		OK:   true,
		ETag: snapshot.Load().ETag,
	})
}

// RebuildSnapshot loads flags for env and swaps the atomic snapshot.
func (s *Server) RebuildSnapshot(ctx context.Context, env string) error {
	flags, err := s.store.GetAllFlags(ctx, env)
	if err != nil {
		return err
	}
	snap := snapshot.BuildFromFlags(flags)
	snapshot.Update(snap)
	telemetry.SnapshotFlags.Set(float64(len(snap.Flags)))
	return nil
}

// ---- middleware & helpers ----

func (s *Server) authAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		got := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer"))
		got = strings.TrimSpace(strings.TrimPrefix(got, "Bearer"))
		if got == "" {
			UnauthorizedError(w, r, "Missing bearer token")
			return
		}
		// constant-time compare
		if subtle.ConstantTimeCompare([]byte(got), []byte(s.adminAPIKey)) != 1 {
			ForbiddenError(w, r, "Invalid token")
			return
		}
		next.ServeHTTP(w, r)
	}
}

// auditLog logs an audit event (convenience method for backward compatibility during migration).
// Consider using audit.NewEventBuilder(r) directly with the builder pattern for new code.
func (s *Server) auditLog(r *http.Request, action, resourceType, resourceID, environment string, beforeState, afterState, changes map[string]any, status, errorMsg string) {
	if s.auditService == nil {
		return // No audit service available
	}

	builder := audit.NewEventBuilder(r).
		ForResource(resourceType, resourceID).
		WithAction(action).
		WithEnvironment(environment).
		WithBeforeState(beforeState).
		WithAfterState(afterState).
		WithChanges(changes)

	if status == audit.StatusFailure && errorMsg != "" {
		builder = builder.Failure(errorMsg)
	}

	event := builder.Build()
	s.auditService.Log(event)
}

// dispatchWebhookEvent dispatches a webhook event for flag changes using the EventBuilder pattern.
// Event type (created/updated/deleted) is automatically determined based on before/after states.
func (s *Server) dispatchWebhookEvent(r *http.Request, key, env string, beforeState, afterState, changes map[string]any) {
	if s.webhookDispatcher == nil {
		return // No webhook dispatcher available
	}

	// Build and dispatch event using fluent API
	// Event type is automatically determined based on states
	event := webhook.NewEventBuilder(r).
		ForFlag(key, env).
		WithStates(beforeState, afterState).
		WithChanges(changes).
		Build()

	// Dispatch asynchronously (non-blocking)
	s.webhookDispatcher.Dispatch(event)
}
