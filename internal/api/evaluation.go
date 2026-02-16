package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/TimurManjosov/goflagship/internal/engine"
	"github.com/TimurManjosov/goflagship/internal/snapshot"
	"github.com/TimurManjosov/goflagship/internal/store"
)

var evaluationSnapshotMu sync.RWMutex

// handleContextEvaluate handles POST /v1/evaluate.
// POST is used to support complex JSON context payloads while keeping evaluation stateless.
func (s *Server) handleContextEvaluate(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxFlagRequestBodySize)
	defer r.Body.Close()

	var req EvaluationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			RequestTooLargeError(w, r, "Request body exceeds 1MB limit")
			return
		}
		BadRequestError(w, r, ErrCodeInvalidJSON, "Invalid JSON: "+err.Error())
		return
	}

	if isEmptyEvaluationContext(req.Context) {
		BadRequestErrorWithFields(w, r, ErrCodeMissingField, "Missing required field", map[string]string{
			"context": "context is required",
		})
		return
	}

	ctx := toUserContext(req.Context)
	flagKey := strings.TrimSpace(req.FlagKey)
	if flagKey != "" {
		s.evaluateSingleFlag(w, r, flagKey, &ctx)
		return
	}

	s.evaluateAllFlags(w, &ctx)
}

func (s *Server) evaluateSingleFlag(w http.ResponseWriter, r *http.Request, flagKey string, ctx *engine.UserContext) {
	evaluationSnapshotMu.RLock()
	snap := snapshot.Load()
	flag, exists := snap.Flags[flagKey]
	evaluationSnapshotMu.RUnlock()
	if !exists {
		NotFoundError(w, r, "Flag '"+flagKey+"' not found")
		return
	}

	result := evaluateSnapshotFlag(flag, ctx)
	writeJSON(w, http.StatusOK, EvaluationResponse{
		Results: []FlagResult{result},
	})
}

func (s *Server) evaluateAllFlags(w http.ResponseWriter, ctx *engine.UserContext) {
	evaluationSnapshotMu.RLock()
	snap := snapshot.Load()
	keys := make([]string, 0, len(snap.Flags))
	for key := range snap.Flags {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	results := make([]FlagResult, 0, len(keys))
	for _, key := range keys {
		results = append(results, evaluateSnapshotFlag(snap.Flags[key], ctx))
	}
	evaluationSnapshotMu.RUnlock()

	writeJSON(w, http.StatusOK, EvaluationResponse{
		Results: results,
	})
}

func evaluateSnapshotFlag(flag snapshot.FlagView, ctx *engine.UserContext) FlagResult {
	evaluation := engine.Evaluate(toStoreFlag(flag), ctx)
	return FlagResult{
		Key:     flag.Key,
		Enabled: evaluation.Reason != string(engine.ReasonDisabled),
		Value:   evaluation.Value,
		Variant: evaluation.Variant,
		Reason:  evaluation.Reason,
	}
}

func isEmptyEvaluationContext(ctx EvaluationContextDTO) bool {
	return strings.TrimSpace(ctx.ID) == "" &&
		strings.TrimSpace(ctx.Email) == "" &&
		strings.TrimSpace(ctx.Country) == "" &&
		len(ctx.Properties) == 0
}

func toUserContext(dto EvaluationContextDTO) engine.UserContext {
	return engine.UserContext{
		ID:         dto.ID,
		Email:      dto.Email,
		Country:    dto.Country,
		Properties: dto.Properties,
	}
}

func toStoreFlag(flag snapshot.FlagView) *store.Flag {
	variants := make([]store.Variant, 0, len(flag.Variants))
	for _, variant := range flag.Variants {
		variants = append(variants, store.Variant{
			Name:   variant.Name,
			Weight: variant.Weight,
			Config: variant.Config,
		})
	}

	return &store.Flag{
		Key:            flag.Key,
		Enabled:        flag.Enabled,
		Config:         flag.Config,
		TargetingRules: flag.TargetingRules,
		Variants:       variants,
	}
}
