// Package api provides HTTP handlers and middleware for the flagship feature flag service.
//
// Flag Evaluation Flow (POST /v1/flags/evaluate):
//
//  1. Parse and validate request (user ID required, optional flag keys filter)
//  2. Load current snapshot from memory (thread-safe atomic read)
//  3. For each flag in snapshot (or filtered subset):
//     a. Check if flag is enabled (if not, return enabled=false)
//     b. Evaluate targeting expression against user context (using JSON Logic)
//     c. Evaluate rollout percentage with deterministic bucketing (hash-based)
//     d. Evaluate variants for A/B testing (if configured)
//  4. Build response with evaluation results and ETag for caching
//  5. Return response (evaluation is a read operation, no audit logging)
//
// The evaluation is stateless and read-only, making it safe for high-concurrency workloads.
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
		BadRequestError(w, r, ErrCodeInvalidJSON, "Invalid JSON: "+err.Error())
		return
	}

	// Validate with field-level errors
	errors := make(map[string]string)
	if req.User == nil {
		errors["user"] = "User is required"
	} else if strings.TrimSpace(req.User.ID) == "" {
		errors["user.id"] = "User ID is required"
	}

	if len(errors) > 0 {
		ValidationError(w, r, "Validation failed for one or more fields", errors)
		return
	}

	// Build evaluation context and evaluate
	ctx := evaluation.Context{
		UserID:     req.User.ID,
		Attributes: req.User.Attributes,
	}

	s.evaluateAndRespond(w, ctx, req.Keys)
}

// handleEvaluateGET handles GET /v1/flags/evaluate with query parameters
func (s *Server) handleEvaluateGET(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	// Get userId (required)
	userID := strings.TrimSpace(query.Get("userId"))
	if userID == "" {
		BadRequestErrorWithFields(w, r, ErrCodeMissingField, "Missing required parameter", map[string]string{
			"userId": "userId query parameter is required",
		})
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

	// Build evaluation context and evaluate
	ctx := evaluation.Context{
		UserID:     userID,
		Attributes: attributes,
	}

	s.evaluateAndRespond(w, ctx, keys)
}

// evaluateAndRespond performs flag evaluation and writes the JSON response.
// This is shared by both POST and GET evaluation handlers to avoid duplication.
func (s *Server) evaluateAndRespond(w http.ResponseWriter, ctx evaluation.Context, keys []string) {
	// Load current snapshot
	snap := snapshot.Load()

	// Evaluate flags
	results := evaluation.EvaluateAll(snap.Flags, ctx, snap.RolloutSalt, keys)

	// Build and write response
	resp := evaluateResponse{
		Flags:       results,
		ETag:        snap.ETag,
		EvaluatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	writeJSON(w, http.StatusOK, resp)
}
