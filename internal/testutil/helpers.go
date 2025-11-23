package testutil

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/TimurManjosov/goflagship/internal/api"
	"github.com/TimurManjosov/goflagship/internal/store"
)

// NewTestServer creates a test server with in-memory store for testing.
func NewTestServer(t *testing.T, env, adminKey string) (*api.Server, *store.MemoryStore) {
	t.Helper()
	memStore := store.NewMemoryStore()
	server := api.NewServer(memStore, env, adminKey)
	return server, memStore
}

// HTTPRequest is a helper for making test HTTP requests.
type HTTPRequest struct {
	Method  string
	Path    string
	Body    string
	Headers map[string]string
}

// Do executes the HTTP request and returns the response recorder.
func (r *HTTPRequest) Do(t *testing.T, handler http.Handler) *httptest.ResponseRecorder {
	t.Helper()
	var body io.Reader
	if r.Body != "" {
		body = bytes.NewBufferString(r.Body)
	}
	req := httptest.NewRequest(r.Method, r.Path, body)
	if r.Body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range r.Headers {
		req.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

// SeedFlags populates the store with test flags.
func SeedFlags(ctx context.Context, st store.Store, flags []store.UpsertParams) error {
	for _, f := range flags {
		if err := st.UpsertFlag(ctx, f); err != nil {
			return err
		}
	}
	return nil
}
