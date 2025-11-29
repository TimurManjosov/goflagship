package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/TimurManjosov/goflagship/internal/snapshot"
	"github.com/TimurManjosov/goflagship/internal/store"
)

func TestHandleEvaluate_BasicFlag(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "test-key")
	handler := srv.Router()
	ctx := context.Background()

	// Set a consistent rollout salt for deterministic tests
	snapshot.SetRolloutSalt("test-salt")

	// Add a simple enabled flag
	st.UpsertFlag(ctx, store.UpsertParams{
		Key:     "test_flag",
		Enabled: true,
		Rollout: 100,
		Env:     "prod",
	})
	srv.RebuildSnapshot(ctx, "prod")

	body := `{"user": {"id": "user-123"}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/flags/evaluate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp evaluateResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(resp.Flags) != 1 {
		t.Errorf("Expected 1 flag, got %d", len(resp.Flags))
	}
	if resp.Flags[0].Key != "test_flag" {
		t.Errorf("Expected flag key 'test_flag', got '%s'", resp.Flags[0].Key)
	}
	if !resp.Flags[0].Enabled {
		t.Error("Expected flag to be enabled")
	}
	if resp.ETag == "" {
		t.Error("Expected ETag in response")
	}
	if resp.EvaluatedAt == "" {
		t.Error("Expected evaluatedAt in response")
	}
}

func TestHandleEvaluate_DisabledFlag(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "test-key")
	handler := srv.Router()
	ctx := context.Background()

	snapshot.SetRolloutSalt("test-salt")

	// Add a disabled flag
	st.UpsertFlag(ctx, store.UpsertParams{
		Key:     "disabled_flag",
		Enabled: false,
		Rollout: 100,
		Env:     "prod",
	})
	srv.RebuildSnapshot(ctx, "prod")

	body := `{"user": {"id": "user-123"}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/flags/evaluate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	var resp evaluateResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if len(resp.Flags) != 1 {
		t.Errorf("Expected 1 flag, got %d", len(resp.Flags))
	}
	if resp.Flags[0].Enabled {
		t.Error("Expected flag to be disabled")
	}
}

func TestHandleEvaluate_WithExpression(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "test-key")
	handler := srv.Router()
	ctx := context.Background()

	snapshot.SetRolloutSalt("test-salt")

	// Add a flag with expression
	expr := `{"==": [{"var": "plan"}, "premium"]}`
	st.UpsertFlag(ctx, store.UpsertParams{
		Key:        "premium_flag",
		Enabled:    true,
		Rollout:    100,
		Expression: &expr,
		Env:        "prod",
	})
	srv.RebuildSnapshot(ctx, "prod")

	// Test with matching user
	body := `{"user": {"id": "user-123", "attributes": {"plan": "premium"}}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/flags/evaluate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	var resp evaluateResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if !resp.Flags[0].Enabled {
		t.Error("Expected flag to be enabled for premium user")
	}

	// Test with non-matching user
	body = `{"user": {"id": "user-456", "attributes": {"plan": "free"}}}`
	req = httptest.NewRequest(http.MethodPost, "/v1/flags/evaluate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.Flags[0].Enabled {
		t.Error("Expected flag to be disabled for free user")
	}
}

func TestHandleEvaluate_WithConfig(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "test-key")
	handler := srv.Router()
	ctx := context.Background()

	snapshot.SetRolloutSalt("test-salt")

	// Add a flag with config
	st.UpsertFlag(ctx, store.UpsertParams{
		Key:     "config_flag",
		Enabled: true,
		Rollout: 100,
		Config:  map[string]any{"color": "blue", "size": 10},
		Env:     "prod",
	})
	srv.RebuildSnapshot(ctx, "prod")

	body := `{"user": {"id": "user-123"}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/flags/evaluate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	var resp evaluateResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.Flags[0].Config["color"] != "blue" {
		t.Errorf("Expected color 'blue', got %v", resp.Flags[0].Config["color"])
	}
}

func TestHandleEvaluate_FilterByKeys(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "test-key")
	handler := srv.Router()
	ctx := context.Background()

	snapshot.SetRolloutSalt("test-salt")

	// Add multiple flags
	st.UpsertFlag(ctx, store.UpsertParams{Key: "flag1", Enabled: true, Rollout: 100, Env: "prod"})
	st.UpsertFlag(ctx, store.UpsertParams{Key: "flag2", Enabled: true, Rollout: 100, Env: "prod"})
	st.UpsertFlag(ctx, store.UpsertParams{Key: "flag3", Enabled: true, Rollout: 100, Env: "prod"})
	srv.RebuildSnapshot(ctx, "prod")

	// Request only specific flags
	body := `{"user": {"id": "user-123"}, "keys": ["flag1", "flag3"]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/flags/evaluate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	var resp evaluateResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if len(resp.Flags) != 2 {
		t.Errorf("Expected 2 flags, got %d", len(resp.Flags))
	}

	// Check that only requested flags are present
	foundFlags := make(map[string]bool)
	for _, f := range resp.Flags {
		foundFlags[f.Key] = true
	}
	if !foundFlags["flag1"] || !foundFlags["flag3"] {
		t.Error("Expected flag1 and flag3 in response")
	}
	if foundFlags["flag2"] {
		t.Error("Did not expect flag2 in response")
	}
}

func TestHandleEvaluate_MissingUser(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "test-key")
	handler := srv.Router()

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/v1/flags/evaluate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}
}

func TestHandleEvaluate_MissingUserID(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "test-key")
	handler := srv.Router()

	body := `{"user": {"id": ""}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/flags/evaluate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}
}

func TestHandleEvaluate_InvalidJSON(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "test-key")
	handler := srv.Router()

	req := httptest.NewRequest(http.MethodPost, "/v1/flags/evaluate", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}
}

func TestHandleEvaluate_EmptyFlags(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "test-key")
	handler := srv.Router()
	ctx := context.Background()

	snapshot.SetRolloutSalt("test-salt")
	srv.RebuildSnapshot(ctx, "prod")

	body := `{"user": {"id": "user-123"}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/flags/evaluate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	var resp evaluateResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if len(resp.Flags) != 0 {
		t.Errorf("Expected 0 flags, got %d", len(resp.Flags))
	}
}

func TestHandleEvaluateGET_BasicFlag(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "test-key")
	handler := srv.Router()
	ctx := context.Background()

	snapshot.SetRolloutSalt("test-salt")

	// Add a simple enabled flag
	st.UpsertFlag(ctx, store.UpsertParams{
		Key:     "test_flag",
		Enabled: true,
		Rollout: 100,
		Env:     "prod",
	})
	srv.RebuildSnapshot(ctx, "prod")

	req := httptest.NewRequest(http.MethodGet, "/v1/flags/evaluate?userId=user-123", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp evaluateResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(resp.Flags) != 1 {
		t.Errorf("Expected 1 flag, got %d", len(resp.Flags))
	}
	if !resp.Flags[0].Enabled {
		t.Error("Expected flag to be enabled")
	}
}

func TestHandleEvaluateGET_WithAttributes(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "test-key")
	handler := srv.Router()
	ctx := context.Background()

	snapshot.SetRolloutSalt("test-salt")

	// Add a flag with expression
	expr := `{"==": [{"var": "plan"}, "premium"]}`
	st.UpsertFlag(ctx, store.UpsertParams{
		Key:        "premium_flag",
		Enabled:    true,
		Rollout:    100,
		Expression: &expr,
		Env:        "prod",
	})
	srv.RebuildSnapshot(ctx, "prod")

	// Test with matching attributes
	req := httptest.NewRequest(http.MethodGet, "/v1/flags/evaluate?userId=user-123&plan=premium", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	var resp evaluateResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if !resp.Flags[0].Enabled {
		t.Error("Expected flag to be enabled for premium user")
	}

	// Test with non-matching attributes
	req = httptest.NewRequest(http.MethodGet, "/v1/flags/evaluate?userId=user-456&plan=free", nil)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.Flags[0].Enabled {
		t.Error("Expected flag to be disabled for free user")
	}
}

func TestHandleEvaluateGET_FilterByKeys(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "test-key")
	handler := srv.Router()
	ctx := context.Background()

	snapshot.SetRolloutSalt("test-salt")

	// Add multiple flags
	st.UpsertFlag(ctx, store.UpsertParams{Key: "flag1", Enabled: true, Rollout: 100, Env: "prod"})
	st.UpsertFlag(ctx, store.UpsertParams{Key: "flag2", Enabled: true, Rollout: 100, Env: "prod"})
	st.UpsertFlag(ctx, store.UpsertParams{Key: "flag3", Enabled: true, Rollout: 100, Env: "prod"})
	srv.RebuildSnapshot(ctx, "prod")

	// Request only specific flags
	req := httptest.NewRequest(http.MethodGet, "/v1/flags/evaluate?userId=user-123&keys=flag1,flag3", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	var resp evaluateResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if len(resp.Flags) != 2 {
		t.Errorf("Expected 2 flags, got %d", len(resp.Flags))
	}
}

func TestHandleEvaluateGET_MissingUserId(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "test-key")
	handler := srv.Router()

	req := httptest.NewRequest(http.MethodGet, "/v1/flags/evaluate", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}
}

func TestHandleEvaluate_NonExistentKey(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "test-key")
	handler := srv.Router()
	ctx := context.Background()

	snapshot.SetRolloutSalt("test-salt")

	st.UpsertFlag(ctx, store.UpsertParams{Key: "flag1", Enabled: true, Rollout: 100, Env: "prod"})
	srv.RebuildSnapshot(ctx, "prod")

	// Request existing and non-existent flag
	body := `{"user": {"id": "user-123"}, "keys": ["flag1", "nonexistent"]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/flags/evaluate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	var resp evaluateResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	// Should only return existing flag
	if len(resp.Flags) != 1 {
		t.Errorf("Expected 1 flag, got %d", len(resp.Flags))
	}
	if resp.Flags[0].Key != "flag1" {
		t.Errorf("Expected flag1, got %s", resp.Flags[0].Key)
	}
}

func TestHandleEvaluate_WithVariants(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "test-key")
	handler := srv.Router()
	ctx := context.Background()

	snapshot.SetRolloutSalt("test-salt")

	// Add a flag with variants
	st.UpsertFlag(ctx, store.UpsertParams{
		Key:     "variant_flag",
		Enabled: true,
		Rollout: 100,
		Variants: []store.Variant{
			{Name: "control", Weight: 50, Config: map[string]any{"color": "red"}},
			{Name: "treatment", Weight: 50, Config: map[string]any{"color": "blue"}},
		},
		Env: "prod",
	})
	srv.RebuildSnapshot(ctx, "prod")

	body := `{"user": {"id": "user-123"}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/flags/evaluate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	var resp evaluateResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if !resp.Flags[0].Enabled {
		t.Error("Expected flag to be enabled")
	}
	if resp.Flags[0].Variant != "control" && resp.Flags[0].Variant != "treatment" {
		t.Errorf("Expected variant to be 'control' or 'treatment', got '%s'", resp.Flags[0].Variant)
	}
	if resp.Flags[0].Config == nil {
		t.Error("Expected variant config to be present")
	}
}

func TestHandleEvaluate_Deterministic(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "test-key")
	handler := srv.Router()
	ctx := context.Background()

	snapshot.SetRolloutSalt("test-salt")

	// Add a flag with 50% rollout
	st.UpsertFlag(ctx, store.UpsertParams{
		Key:     "rollout_flag",
		Enabled: true,
		Rollout: 50,
		Env:     "prod",
	})
	srv.RebuildSnapshot(ctx, "prod")

	// Same user should get same result
	body := `{"user": {"id": "user-123"}}`

	var results []bool
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/flags/evaluate", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		var resp evaluateResponse
		json.NewDecoder(rr.Body).Decode(&resp)
		results = append(results, resp.Flags[0].Enabled)
	}

	// All results should be the same
	for i := 1; i < len(results); i++ {
		if results[i] != results[0] {
			t.Error("Expected deterministic evaluation results")
		}
	}
}
