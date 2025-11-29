package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/TimurManjosov/goflagship/internal/evaluation"
	"github.com/TimurManjosov/goflagship/internal/snapshot"
)

// evaluateRequest represents the request body for POST /v1/flags/evaluate
type evaluateRequest struct {
	User *evaluateUser `json:"user"`
	Keys []string      `json:"keys,omitempty"`
}

// evaluateUser represents the user context in evaluate request
type evaluateUser struct {
	ID         string         `json:"id"`
	Attributes map[string]any `json:"attributes,omitempty"`
}

// evaluateResponse represents the response for /v1/flags/evaluate
type evaluateResponse struct {
	Flags       []evaluation.Result `json:"flags"`
	ETag        string              `json:"etag"`
	EvaluatedAt string              `json:"evaluatedAt"`
}

// handleEvaluate handles POST /v1/flags/evaluate
func (s *Server) handleEvaluate(w http.ResponseWriter, r *http.Request) {
	var req evaluateRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	// Validate user is provided
	if req.User == nil {
		writeError(w, http.StatusBadRequest, "user is required")
		return
	}

	// Validate user ID is provided
	if strings.TrimSpace(req.User.ID) == "" {
		writeError(w, http.StatusBadRequest, "user.id is required")
		return
	}

	// Build evaluation context
	ctx := evaluation.Context{
		UserID:     req.User.ID,
		Attributes: req.User.Attributes,
	}

	// Load current snapshot
	snap := snapshot.Load()

	// Evaluate flags
	results := evaluation.EvaluateAll(snap.Flags, ctx, snap.RolloutSalt, req.Keys)

	// Build response
	resp := evaluateResponse{
		Flags:       results,
		ETag:        snap.ETag,
		EvaluatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	writeJSON(w, http.StatusOK, resp)
}

// handleEvaluateGET handles GET /v1/flags/evaluate with query parameters
func (s *Server) handleEvaluateGET(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	// Get userId (required)
	userID := strings.TrimSpace(query.Get("userId"))
	if userID == "" {
		writeError(w, http.StatusBadRequest, "userId query parameter is required")
		return
	}

	// Get keys (optional, comma-separated)
	var keys []string
	if keysParam := query.Get("keys"); keysParam != "" {
		keys = strings.Split(keysParam, ",")
		for i := range keys {
			keys[i] = strings.TrimSpace(keys[i])
		}
	}

	// Build attributes from other query params
	attributes := make(map[string]any)
	for key, values := range query {
		// Skip userId and keys parameters
		if key == "userId" || key == "keys" {
			continue
		}
		// Use the first value for each attribute
		if len(values) > 0 {
			attributes[key] = values[0]
		}
	}

	// Build evaluation context
	ctx := evaluation.Context{
		UserID:     userID,
		Attributes: attributes,
	}

	// Load current snapshot
	snap := snapshot.Load()

	// Evaluate flags
	results := evaluation.EvaluateAll(snap.Flags, ctx, snap.RolloutSalt, keys)

	// Build response
	resp := evaluateResponse{
		Flags:       results,
		ETag:        snap.ETag,
		EvaluatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	writeJSON(w, http.StatusOK, resp)
}
