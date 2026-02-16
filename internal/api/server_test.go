package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/TimurManjosov/goflagship/internal/rules"
	"github.com/TimurManjosov/goflagship/internal/snapshot"
	"github.com/TimurManjosov/goflagship/internal/store"
)

func TestHandleHealth(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "test-key")
	handler := srv.Router()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	if rr.Body.String() != "ok" {
		t.Errorf("Expected body 'ok', got %s", rr.Body.String())
	}
}

func TestSnapshotEndpoint_EmptyFlags(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "test-key")
	handler := srv.Router()

	// Initialize empty snapshot
	srv.RebuildSnapshot(context.Background(), "prod")

	req := httptest.NewRequest(http.MethodGet, "/v1/flags/snapshot", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	var snap snapshot.Snapshot
	if err := json.NewDecoder(rr.Body).Decode(&snap); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(snap.Flags) != 0 {
		t.Errorf("Expected 0 flags, got %d", len(snap.Flags))
	}

	etag := rr.Header().Get("ETag")
	if etag == "" {
		t.Error("Expected ETag header to be set")
	}
}

func TestSnapshotEndpoint_WithFlags(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "test-key")
	handler := srv.Router()
	ctx := context.Background()

	// Add flags
	st.UpsertFlag(ctx, store.UpsertParams{
		Key:     "flag1",
		Enabled: true,
		Rollout: 100,
		Env:     "prod",
	})
	srv.RebuildSnapshot(ctx, "prod")

	req := httptest.NewRequest(http.MethodGet, "/v1/flags/snapshot", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	var snap snapshot.Snapshot
	json.NewDecoder(rr.Body).Decode(&snap)

	if len(snap.Flags) != 1 {
		t.Errorf("Expected 1 flag, got %d", len(snap.Flags))
	}
}

func TestSnapshotEndpoint_CacheHeaders(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "test-key")
	handler := srv.Router()
	srv.RebuildSnapshot(context.Background(), "prod")

	req := httptest.NewRequest(http.MethodGet, "/v1/flags/snapshot", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Check cache control headers
	cacheControl := rr.Header().Get("Cache-Control")
	if cacheControl != "no-cache, no-store, must-revalidate" {
		t.Errorf("Expected 'no-cache, no-store, must-revalidate', got %s", cacheControl)
	}

	pragma := rr.Header().Get("Pragma")
	if pragma != "no-cache" {
		t.Errorf("Expected 'no-cache', got %s", pragma)
	}

	expires := rr.Header().Get("Expires")
	if expires != "0" {
		t.Errorf("Expected '0', got %s", expires)
	}
}

func TestSnapshotEndpoint_ETag_NotModified(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "test-key")
	handler := srv.Router()
	ctx := context.Background()

	// Add a flag and rebuild
	st.UpsertFlag(ctx, store.UpsertParams{
		Key:     "test",
		Enabled: true,
		Rollout: 100,
		Env:     "prod",
	})
	srv.RebuildSnapshot(ctx, "prod")

	// First request to get ETag
	req1 := httptest.NewRequest(http.MethodGet, "/v1/flags/snapshot", nil)
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)

	etag := rr1.Header().Get("ETag")
	if etag == "" {
		t.Fatal("ETag not set in response")
	}

	// Second request with If-None-Match
	req2 := httptest.NewRequest(http.MethodGet, "/v1/flags/snapshot", nil)
	req2.Header.Set("If-None-Match", etag)
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusNotModified {
		t.Errorf("Expected status 304, got %d", rr2.Code)
	}

	if rr2.Body.Len() != 0 {
		t.Error("Expected empty body for 304 response")
	}
}

func TestSnapshotEndpoint_ETag_Modified(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "test-key")
	handler := srv.Router()
	ctx := context.Background()

	// Initial state
	srv.RebuildSnapshot(ctx, "prod")
	req1 := httptest.NewRequest(http.MethodGet, "/v1/flags/snapshot", nil)
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)
	oldETag := rr1.Header().Get("ETag")

	// Modify flags
	st.UpsertFlag(ctx, store.UpsertParams{
		Key:     "new_flag",
		Enabled: true,
		Rollout: 100,
		Env:     "prod",
	})
	srv.RebuildSnapshot(ctx, "prod")

	// Request with old ETag
	req2 := httptest.NewRequest(http.MethodGet, "/v1/flags/snapshot", nil)
	req2.Header.Set("If-None-Match", oldETag)
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Errorf("Expected status 200 (modified), got %d", rr2.Code)
	}

	newETag := rr2.Header().Get("ETag")
	if newETag == oldETag {
		t.Error("Expected different ETag after modification")
	}
}

func TestUpsertFlag_Success(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "admin-key")
	handler := srv.Router()

	body := `{
		"key": "test_flag",
		"description": "Test flag",
		"enabled": true,
		"rollout": 50,
		"config": {"color": "red"}
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/flags", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer admin-key")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp upsertResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if !resp.OK {
		t.Error("Expected OK to be true")
	}
	if resp.ETag == "" {
		t.Error("Expected ETag in response")
	}
}

func TestUpsertFlag_InvalidJSON(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "admin-key")
	handler := srv.Router()

	req := httptest.NewRequest(http.MethodPost, "/v1/flags", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer admin-key")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}
}

func TestUpsertFlag_InvalidExpression(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "admin-key")
	handler := srv.Router()

	// Invalid expression (not valid JSON)
	body := `{
		"key": "test_flag",
		"enabled": true,
		"rollout": 100,
		"expression": "not valid json logic"
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/flags", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer admin-key")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestUpsertFlag_ValidExpression(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "admin-key")
	handler := srv.Router()

	// Valid JSON Logic expression
	body := `{
		"key": "test_flag",
		"enabled": true,
		"rollout": 100,
		"expression": "{\"==\": [{\"var\": \"plan\"}, \"premium\"]}"
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/flags", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer admin-key")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestUpsertFlag_MissingKey(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "admin-key")
	handler := srv.Router()

	body := `{
		"enabled": true,
		"rollout": 50
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/flags", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer admin-key")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}
}

func TestUpsertFlag_InvalidRollout(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "admin-key")
	handler := srv.Router()

	tests := []struct {
		name    string
		rollout int32
	}{
		{"negative", -1},
		{"too high", 101},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := fmt.Sprintf(`{"key": "test", "enabled": true, "rollout": %d}`, tt.rollout)

			req := httptest.NewRequest(http.MethodPost, "/v1/flags", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer admin-key")
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Errorf("Expected status 400, got %d", rr.Code)
			}
		})
	}
}

func TestUpsertFlag_Unauthorized(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "admin-key")
	handler := srv.Router()

	body := `{"key": "test", "enabled": true, "rollout": 50}`

	req := httptest.NewRequest(http.MethodPost, "/v1/flags", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	// No Authorization header
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rr.Code)
	}
}

func TestUpsertFlag_InvalidToken(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "admin-key")
	handler := srv.Router()

	body := `{"key": "test", "enabled": true, "rollout": 50}`

	req := httptest.NewRequest(http.MethodPost, "/v1/flags", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer wrong-key")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rr.Code)
	}
}

func TestUpsertFlag_WithTargetingRules(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "admin-key")
	handler := srv.Router()

	body := `{
		"key": "test_flag",
		"enabled": true,
		"rollout": 100,
		"targeting_rules": [{
			"id": "rule-1",
			"conditions": [{"property":"plan","operator":"eq","value":"premium"}],
			"distribution": {"on": 100}
		}]
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/flags", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer admin-key")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	flag, err := st.GetFlagByKey(context.Background(), "test_flag")
	if err != nil {
		t.Fatalf("Failed to load stored flag: %v", err)
	}
	if len(flag.TargetingRules) != 1 {
		t.Fatalf("Expected 1 targeting rule, got %d", len(flag.TargetingRules))
	}
}

func TestUpsertFlag_InvalidTargetingRules(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "admin-key")
	handler := srv.Router()

	body := `{
		"key": "test_flag",
		"enabled": true,
		"rollout": 100,
		"targeting_rules": [{
			"id": "rule-1",
			"conditions": [{"property":"plan","operator":"invalid","value":"premium"}],
			"distribution": {"on": 100}
		}]
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/flags", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer admin-key")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("Expected status 400, got %d: %s", rr.Code, rr.Body.String())
	}

	var errResp ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&errResp); err != nil {
		t.Fatalf("Failed to decode error response: %v", err)
	}
	if errResp.Message != "invalid targeting_rules" {
		t.Fatalf("Expected invalid targeting_rules message, got %q", errResp.Message)
	}
	if _, ok := errResp.Fields["targeting_rules[0]"]; !ok {
		t.Fatalf("Expected targeting_rules[0] field in validation response, got %+v", errResp.Fields)
	}
}

func TestUpsertFlag_RequestTooLarge(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "admin-key")
	handler := srv.Router()

	tooLarge := fmt.Sprintf(`{"key":"big","enabled":true,"rollout":100,"description":"%s"}`, strings.Repeat("x", 1<<20))
	req := httptest.NewRequest(http.MethodPost, "/v1/flags", bytes.NewBufferString(tooLarge))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer admin-key")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("Expected status 413, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestUpdateFlag_WithTargetingRules(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "admin-key")
	handler := srv.Router()
	ctx := context.Background()

	if err := st.UpsertFlag(ctx, store.UpsertParams{Key: "test_flag", Enabled: true, Rollout: 100, Env: "prod"}); err != nil {
		t.Fatalf("Failed to seed flag: %v", err)
	}

	body := `{
		"enabled": true,
		"rollout": 100,
		"targeting_rules": [{
			"id": "rule-1",
			"conditions": [{"property":"plan","operator":"eq","value":"premium"}],
			"distribution": {"on": 100}
		}]
	}`
	req := httptest.NewRequest(http.MethodPut, "/v1/flags/test_flag", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer admin-key")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	flag, err := st.GetFlagByKey(ctx, "test_flag")
	if err != nil {
		t.Fatalf("Failed to load updated flag: %v", err)
	}
	if len(flag.TargetingRules) != 1 {
		t.Fatalf("Expected 1 targeting rule after update, got %d", len(flag.TargetingRules))
	}
}

func TestGetAndListFlags_IncludeTargetingRules(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "admin-key")
	handler := srv.Router()
	ctx := context.Background()

	if err := st.UpsertFlag(ctx, store.UpsertParams{
		Key:     "test_flag",
		Enabled: true,
		Rollout: 100,
		Env:     "prod",
		TargetingRules: []rules.Rule{{
			ID: "rule-1",
			Conditions: []rules.Condition{{
				Property: "plan",
				Operator: rules.OpEq,
				Value:    "premium",
			}},
			Distribution: map[string]int{"on": 100},
		}},
	}); err != nil {
		t.Fatalf("Failed to seed flag: %v", err)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/v1/flags/test_flag?env=prod", nil)
	getReq.Header.Set("Authorization", "Bearer admin-key")
	getRR := httptest.NewRecorder()
	handler.ServeHTTP(getRR, getReq)
	if getRR.Code != http.StatusOK {
		t.Fatalf("Expected GET status 200, got %d: %s", getRR.Code, getRR.Body.String())
	}

	var getResp flagResponse
	if err := json.NewDecoder(getRR.Body).Decode(&getResp); err != nil {
		t.Fatalf("Failed to decode GET response: %v", err)
	}
	if len(getResp.TargetingRules) != 1 {
		t.Fatalf("Expected 1 targeting rule in GET response, got %d", len(getResp.TargetingRules))
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/flags?env=prod", nil)
	listReq.Header.Set("Authorization", "Bearer admin-key")
	listRR := httptest.NewRecorder()
	handler.ServeHTTP(listRR, listReq)
	if listRR.Code != http.StatusOK {
		t.Fatalf("Expected list status 200, got %d: %s", listRR.Code, listRR.Body.String())
	}

	var listResp listFlagsResponse
	if err := json.NewDecoder(listRR.Body).Decode(&listResp); err != nil {
		t.Fatalf("Failed to decode list response: %v", err)
	}
	if len(listResp.Flags) == 0 || len(listResp.Flags[0].TargetingRules) != 1 {
		t.Fatalf("Expected targeting rules in list response, got %+v", listResp.Flags)
	}
}

func TestDeleteFlag_Success(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "admin-key")
	handler := srv.Router()
	ctx := context.Background()

	// Create a flag first
	st.UpsertFlag(ctx, store.UpsertParams{
		Key:     "to_delete",
		Enabled: true,
		Rollout: 100,
		Env:     "prod",
	})
	srv.RebuildSnapshot(ctx, "prod")

	// Delete it
	req := httptest.NewRequest(http.MethodDelete, "/v1/flags?key=to_delete&env=prod", nil)
	req.Header.Set("Authorization", "Bearer admin-key")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	var resp upsertResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if !resp.OK {
		t.Error("Expected OK to be true")
	}
}

func TestDeleteFlag_MissingKey(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "admin-key")
	handler := srv.Router()

	req := httptest.NewRequest(http.MethodDelete, "/v1/flags?env=prod", nil)
	req.Header.Set("Authorization", "Bearer admin-key")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}
}

func TestDeleteFlag_MissingEnv(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "admin-key")
	handler := srv.Router()

	req := httptest.NewRequest(http.MethodDelete, "/v1/flags?key=test", nil)
	req.Header.Set("Authorization", "Bearer admin-key")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}
}

func TestDeleteFlag_Idempotent(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "admin-key")
	handler := srv.Router()

	// Delete non-existent flag (should succeed)
	req := httptest.NewRequest(http.MethodDelete, "/v1/flags?key=nonexistent&env=prod", nil)
	req.Header.Set("Authorization", "Bearer admin-key")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200 (idempotent), got %d", rr.Code)
	}
}

func TestDeleteFlag_Unauthorized(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "admin-key")
	handler := srv.Router()

	req := httptest.NewRequest(http.MethodDelete, "/v1/flags?key=test&env=prod", nil)
	// No Authorization header
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rr.Code)
	}
}

func TestETagChangesAfterMutation(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "admin-key")
	handler := srv.Router()
	ctx := context.Background()

	// Initial snapshot
	srv.RebuildSnapshot(ctx, "prod")
	req1 := httptest.NewRequest(http.MethodGet, "/v1/flags/snapshot", nil)
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)
	etag1 := rr1.Header().Get("ETag")

	// Create flag
	body := `{"key": "new_flag", "enabled": true, "rollout": 100}`
	req2 := httptest.NewRequest(http.MethodPost, "/v1/flags", bytes.NewBufferString(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer admin-key")
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	// Get new snapshot
	req3 := httptest.NewRequest(http.MethodGet, "/v1/flags/snapshot", nil)
	rr3 := httptest.NewRecorder()
	handler.ServeHTTP(rr3, req3)
	etag2 := rr3.Header().Get("ETag")

	if etag1 == etag2 {
		t.Error("Expected ETag to change after flag creation")
	}

	// Update flag
	body = `{"key": "new_flag", "enabled": false, "rollout": 50}`
	req4 := httptest.NewRequest(http.MethodPost, "/v1/flags", bytes.NewBufferString(body))
	req4.Header.Set("Content-Type", "application/json")
	req4.Header.Set("Authorization", "Bearer admin-key")
	rr4 := httptest.NewRecorder()
	handler.ServeHTTP(rr4, req4)

	// Get updated snapshot
	req5 := httptest.NewRequest(http.MethodGet, "/v1/flags/snapshot", nil)
	rr5 := httptest.NewRecorder()
	handler.ServeHTTP(rr5, req5)
	etag3 := rr5.Header().Get("ETag")

	if etag2 == etag3 {
		t.Error("Expected ETag to change after flag update")
	}

	// Delete flag
	req6 := httptest.NewRequest(http.MethodDelete, "/v1/flags?key=new_flag&env=prod", nil)
	req6.Header.Set("Authorization", "Bearer admin-key")
	rr6 := httptest.NewRecorder()
	handler.ServeHTTP(rr6, req6)

	// Get final snapshot
	req7 := httptest.NewRequest(http.MethodGet, "/v1/flags/snapshot", nil)
	rr7 := httptest.NewRecorder()
	handler.ServeHTTP(rr7, req7)
	etag4 := rr7.Header().Get("ETag")

	if etag3 == etag4 {
		t.Error("Expected ETag to change after flag deletion")
	}
}

func TestSnapshot_EnvironmentFiltering(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "admin-key")
	handler := srv.Router()
	ctx := context.Background()

	// Add flags to different environments
	st.UpsertFlag(ctx, store.UpsertParams{Key: "prod_flag", Enabled: true, Rollout: 100, Env: "prod"})
	st.UpsertFlag(ctx, store.UpsertParams{Key: "dev_flag", Enabled: true, Rollout: 100, Env: "dev"})

	// Rebuild for prod
	srv.RebuildSnapshot(ctx, "prod")

	req := httptest.NewRequest(http.MethodGet, "/v1/flags/snapshot", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	var snap snapshot.Snapshot
	json.NewDecoder(rr.Body).Decode(&snap)

	// Should only have prod flag
	if len(snap.Flags) != 1 {
		t.Errorf("Expected 1 flag, got %d", len(snap.Flags))
	}

	if _, ok := snap.Flags["prod_flag"]; !ok {
		t.Error("Expected prod_flag in snapshot")
	}

	if _, ok := snap.Flags["dev_flag"]; ok {
		t.Error("Did not expect dev_flag in prod snapshot")
	}
}
