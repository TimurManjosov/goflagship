package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgtype"

	dbgen "github.com/TimurManjosov/goflagship/internal/db/gen"
	"github.com/TimurManjosov/goflagship/internal/repo"
	"github.com/TimurManjosov/goflagship/internal/snapshot"
)

type Server struct {
	repo        *repo.Repo
	env         string
	adminAPIKey string
}

func NewServer(r *repo.Repo, env, adminKey string) *Server {
	return &Server{repo: r, env: env, adminAPIKey: adminKey}
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Recoverer)
	r.Use(middleware.Timeout(5 * time.Second))

	// health
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// public: snapshot (ETag)
	r.Get("/v1/flags/snapshot", func(w http.ResponseWriter, req *http.Request) {
		snap := snapshot.Load()
		if inm := req.Header.Get("If-None-Match"); inm != "" && inm == snap.ETag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("ETag", snap.ETag)
		_ = json.NewEncoder(w).Encode(snap)
	})

	// admin (protected): upsert flag
	r.Post("/v1/flags", s.authAdmin(s.handleUpsertFlag))

	return r
}

// ---- handlers ----

type upsertRequest struct {
	Key         string                 `json:"key"`
	Description string                 `json:"description"`
	Enabled     bool                   `json:"enabled"`
	Rollout     int32                  `json:"rollout"`
	Expression  *string                `json:"expression,omitempty"`
	Config      map[string]any         `json:"config,omitempty"`
	Env         *string                `json:"env,omitempty"` // defaults to s.env
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

	// encode config -> []byte
	var cfgBytes []byte
	if req.Config != nil {
		b, err := json.Marshal(req.Config)
		if err != nil {
			writeError(w, http.StatusBadRequest, "config must be JSON object")
			return
		}
		cfgBytes = b
	} else {
		cfgBytes = []byte("{}")
	}

	// upsert via sqlc
	params := dbgen.UpsertFlagParams{
		Key:        req.Key,
		Description: pgtype.Text{String: req.Description, Valid: true},
		Enabled:    req.Enabled,
		Rollout:    req.Rollout,
		Expression: req.Expression, // *string ok
		Config:     cfgBytes,       // []byte
		Env:        env,
	}
	if err := s.repo.UpsertFlag(r.Context(), params); err != nil {
		writeError(w, http.StatusInternalServerError, "db upsert failed")
		return
	}

	// rebuild in-memory snapshot (read fresh rows for env)
	if err := s.RebuildSnapshot(r.Context(), env); err != nil {
		writeError(w, http.StatusInternalServerError, "snapshot rebuild failed")
		return
	}

	// respond with new ETag
	writeJSON(w, http.StatusOK, upsertResponse{
		OK:   true,
		ETag: snapshot.Load().ETag,
	})
}

// RebuildSnapshot loads flags for env and swaps the atomic snapshot.
func (s *Server) RebuildSnapshot(ctx context.Context, env string) error {
	rows, err := s.repo.GetAllFlags(ctx, env)
	if err != nil {
		return err
	}
	snap := snapshot.BuildFromRows(rows)
	snapshot.Update(snap)
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
