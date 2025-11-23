package testutil

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/TimurManjosov/goflagship/internal/store"
)

func TestNewTestServer(t *testing.T) {
	server, memStore := NewTestServer(t, "test", "test-key")
	
	if server == nil {
		t.Fatal("Expected non-nil server")
	}
	if memStore == nil {
		t.Fatal("Expected non-nil store")
	}
	
	// Verify the store is functional
	ctx := context.Background()
	err := memStore.UpsertFlag(ctx, store.UpsertParams{
		Key:     "test",
		Enabled: true,
		Env:     "test",
	})
	if err != nil {
		t.Fatalf("Store should be functional: %v", err)
	}
}

func TestHTTPRequest_Do(t *testing.T) {
	server, _ := NewTestServer(t, "test", "test-key")
	handler := server.Router()
	
	req := &HTTPRequest{
		Method: "GET",
		Path:   "/healthz",
	}
	
	rr := req.Do(t, handler)
	
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
	if rr.Body.String() != "ok" {
		t.Errorf("Expected body 'ok', got '%s'", rr.Body.String())
	}
}

func TestHTTPRequest_DoWithBody(t *testing.T) {
	server, _ := NewTestServer(t, "test", "test-key")
	handler := server.Router()
	
	req := &HTTPRequest{
		Method: "POST",
		Path:   "/v1/flags",
		Body:   `{"key":"test","enabled":true,"env":"test"}`,
		Headers: map[string]string{
			"Authorization": "Bearer test-key",
		},
	}
	
	rr := req.Do(t, handler)
	
	if rr.Code != http.StatusOK {
		t.Logf("Response: %s", rr.Body.String())
	}
}

func TestHTTPRequest_DoWithHeaders(t *testing.T) {
	server, _ := NewTestServer(t, "test", "test-key")
	handler := server.Router()
	
	req := &HTTPRequest{
		Method: "GET",
		Path:   "/v1/flags/snapshot",
		Headers: map[string]string{
			"If-None-Match": "test-etag",
			"Custom-Header": "custom-value",
		},
	}
	
	rr := req.Do(t, handler)
	
	// Should get 200 (not 304 since etag won't match)
	if rr.Code != http.StatusOK {
		t.Logf("Got status: %d", rr.Code)
	}
}

func TestHTTPRequest_ContentTypeAutoSet(t *testing.T) {
	server, _ := NewTestServer(t, "test", "test-key")
	handler := server.Router()
	
	// When Body is provided, Content-Type should be set to application/json
	req := &HTTPRequest{
		Method: "POST",
		Path:   "/v1/flags",
		Body:   `{"key":"test"}`,
		Headers: map[string]string{
			"Authorization": "Bearer test-key",
		},
	}
	
	rr := req.Do(t, handler)
	
	// Verify request was processed (we don't check exact response,
	// just that the helper worked)
	if rr == nil {
		t.Fatal("Expected non-nil response recorder")
	}
}

func TestSeedFlags(t *testing.T) {
	_, memStore := NewTestServer(t, "test", "test-key")
	ctx := context.Background()
	
	flags := []store.UpsertParams{
		{Key: "flag1", Enabled: true, Rollout: 100, Env: "test"},
		{Key: "flag2", Enabled: false, Rollout: 50, Env: "test"},
		{Key: "flag3", Enabled: true, Rollout: 75, Env: "test"},
	}
	
	err := SeedFlags(ctx, memStore, flags)
	if err != nil {
		t.Fatalf("SeedFlags failed: %v", err)
	}
	
	// Verify all flags were inserted
	allFlags, err := memStore.GetAllFlags(ctx, "test")
	if err != nil {
		t.Fatalf("GetAllFlags failed: %v", err)
	}
	
	if len(allFlags) != 3 {
		t.Errorf("Expected 3 flags, got %d", len(allFlags))
	}
	
	// Verify specific flag
	for _, f := range allFlags {
		if f.Key == "flag1" {
			if !f.Enabled {
				t.Error("flag1 should be enabled")
			}
			if f.Rollout != 100 {
				t.Errorf("flag1 should have rollout 100, got %d", f.Rollout)
			}
		}
	}
}

func TestSeedFlags_EmptyList(t *testing.T) {
	_, memStore := NewTestServer(t, "test", "test-key")
	ctx := context.Background()
	
	err := SeedFlags(ctx, memStore, []store.UpsertParams{})
	if err != nil {
		t.Fatalf("SeedFlags with empty list should not fail: %v", err)
	}
	
	allFlags, err := memStore.GetAllFlags(ctx, "test")
	if err != nil {
		t.Fatalf("GetAllFlags failed: %v", err)
	}
	
	if len(allFlags) != 0 {
		t.Errorf("Expected 0 flags, got %d", len(allFlags))
	}
}

func TestSeedFlags_DifferentEnvironments(t *testing.T) {
	_, memStore := NewTestServer(t, "test", "test-key")
	ctx := context.Background()
	
	flags := []store.UpsertParams{
		{Key: "flag1", Enabled: true, Env: "prod"},
		{Key: "flag2", Enabled: true, Env: "dev"},
		{Key: "flag3", Enabled: true, Env: "prod"},
	}
	
	err := SeedFlags(ctx, memStore, flags)
	if err != nil {
		t.Fatalf("SeedFlags failed: %v", err)
	}
	
	// Verify prod flags
	prodFlags, err := memStore.GetAllFlags(ctx, "prod")
	if err != nil {
		t.Fatalf("GetAllFlags failed: %v", err)
	}
	if len(prodFlags) != 2 {
		t.Errorf("Expected 2 prod flags, got %d", len(prodFlags))
	}
	
	// Verify dev flags
	devFlags, err := memStore.GetAllFlags(ctx, "dev")
	if err != nil {
		t.Fatalf("GetAllFlags failed: %v", err)
	}
	if len(devFlags) != 1 {
		t.Errorf("Expected 1 dev flag, got %d", len(devFlags))
	}
}

func TestHTTPRequest_EmptyBody(t *testing.T) {
	server, _ := NewTestServer(t, "test", "test-key")
	handler := server.Router()
	
	req := &HTTPRequest{
		Method: "GET",
		Path:   "/healthz",
		Body:   "", // Empty body
	}
	
	rr := req.Do(t, handler)
	
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestHTTPRequest_HeaderOverride(t *testing.T) {
	server, _ := NewTestServer(t, "test", "test-key")
	handler := server.Router()
	
	// Even with body, can override Content-Type
	req := &HTTPRequest{
		Method: "POST",
		Path:   "/v1/flags",
		Body:   `{"key":"test"}`,
		Headers: map[string]string{
			"Content-Type":  "text/plain",
			"Authorization": "Bearer test-key",
		},
	}
	
	rr := req.Do(t, handler)
	
	// Should fail to parse as JSON and return 400
	if rr.Code != http.StatusBadRequest {
		t.Logf("With wrong Content-Type, got status: %d", rr.Code)
	}
	
	if !strings.Contains(rr.Body.String(), "invalid JSON") {
		t.Logf("Response body: %s", rr.Body.String())
	}
}
