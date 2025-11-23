package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/TimurManjosov/goflagship/internal/auth"
	"github.com/TimurManjosov/goflagship/internal/telemetry"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"

	"github.com/TimurManjosov/goflagship/internal/snapshot"
	"github.com/TimurManjosov/goflagship/internal/store"
)

type Server struct {
	store       store.Store
	env         string
	adminAPIKey string
	auth        *auth.Authenticator
}

func NewServer(s store.Store, env, adminKey string) *Server {
	// Create authenticator with key store
	var keyStore auth.KeyStore
	if pgStore, ok := s.(auth.KeyStore); ok {
		keyStore = pgStore
	}

	authenticator := auth.NewAuthenticator(keyStore, adminKey)

	return &Server{
		store:       s,
		env:         env,
		adminAPIKey: adminKey,
		auth:        authenticator,
	}
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
		r.Route("/v1/flags", func(r chi.Router) {
			r.Post("/", s.authAdmin(s.handleUpsertFlag))
			r.Delete("/", s.authAdmin(s.handleDeleteFlag))
		})

		// Admin API key management routes (superadmin only)
		r.Route("/v1/admin/keys", func(r chi.Router) {
			r.Post("/", s.authAdmin(s.handleCreateAPIKey))
			r.Get("/", s.authAdmin(s.handleListAPIKeys))
			r.Delete("/{id}", s.authAdmin(s.handleRevokeAPIKey))
		})

		// Audit logs route (admin+)
		r.Get("/v1/admin/audit-logs", s.authAdmin(s.handleListAuditLogs))
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

type upsertRequest struct {
	Key         string         `json:"key"`
	Description string         `json:"description"`
	Enabled     bool           `json:"enabled"`
	Rollout     int32          `json:"rollout"`
	Expression  *string        `json:"expression,omitempty"`
	Config      map[string]any `json:"config,omitempty"`
	Env         *string        `json:"env,omitempty"` // defaults to s.env
}

type upsertResponse struct {
	OK   bool   `json:"ok"`
	ETag string `json:"etag"`
}

func (s *Server) handleUpsertFlag(w http.ResponseWriter, r *http.Request) {
	var req upsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	// default env
	env := s.env
	if req.Env != nil && strings.TrimSpace(*req.Env) != "" {
		env = strings.TrimSpace(*req.Env)
	}

	// validation
	if strings.TrimSpace(req.Key) == "" {
		writeError(w, http.StatusBadRequest, "key is required")
		return
	}
	if req.Rollout < 0 || req.Rollout > 100 {
		writeError(w, http.StatusBadRequest, "rollout must be 0..100")
		return
	}

	// upsert via store
	params := store.UpsertParams{
		Key:         req.Key,
		Description: req.Description,
		Enabled:     req.Enabled,
		Rollout:     req.Rollout,
		Expression:  req.Expression,
		Config:      req.Config,
		Env:         env,
	}
	if err := s.store.UpsertFlag(r.Context(), params); err != nil {
		writeError(w, http.StatusInternalServerError, "store upsert failed")
		return
	}

	// rebuild in-memory snapshot (read fresh rows for env)
	if err := s.RebuildSnapshot(r.Context(), env); err != nil {
		writeError(w, http.StatusInternalServerError, "snapshot rebuild failed")
		return
	}

	// Log the action
	s.auditLog(r, "upsert_flag", fmt.Sprintf("flags/%s/%s", env, req.Key), http.StatusOK)

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

	// Validate required parameters
	if key == "" {
		writeError(w, http.StatusBadRequest, "key query parameter is required")
		return
	}
	if env == "" {
		writeError(w, http.StatusBadRequest, "env query parameter is required")
		return
	}

	// Delete from store
	if err := s.store.DeleteFlag(r.Context(), key, env); err != nil {
		writeError(w, http.StatusInternalServerError, "store delete failed")
		return
	}

	// Rebuild snapshot
	if err := s.RebuildSnapshot(r.Context(), env); err != nil {
		writeError(w, http.StatusInternalServerError, "snapshot rebuild failed")
		return
	}

	// Log the action
	s.auditLog(r, "delete_flag", fmt.Sprintf("flags/%s/%s", env, key), http.StatusOK)

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
			writeError(w, http.StatusUnauthorized, "missing bearer token")
			return
		}
		// constant-time compare
		if subtle.ConstantTimeCompare([]byte(got), []byte(s.adminAPIKey)) != 1 {
			writeError(w, http.StatusForbidden, "invalid token")
			return
		}
		next.ServeHTTP(w, r)
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]any{
		"error":   http.StatusText(code),
		"message": msg,
	})
}
