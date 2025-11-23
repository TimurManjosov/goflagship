package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/TimurManjosov/goflagship/internal/snapshot"
	"github.com/TimurManjosov/goflagship/internal/store"
)

func TestConcurrent_FlagUpdates(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "admin-key")
	handler := srv.Router()
	ctx := context.Background()

	srv.RebuildSnapshot(ctx, "prod")

	var wg sync.WaitGroup
	numFlags := 50

	// Create multiple flags concurrently
	for i := 0; i < numFlags; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()

			body := fmt.Sprintf(`{
				"key": "flag_%d",
				"enabled": true,
				"rollout": %d
			}`, n, (n%100)+1)

			req := httptest.NewRequest(http.MethodPost, "/v1/flags", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer admin-key")
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("Failed to create flag_%d: status %d", n, rr.Code)
			}
		}(i)
	}

	wg.Wait()

	// Verify all flags were created
	flags, err := st.GetAllFlags(ctx, "prod")
	if err != nil {
		t.Fatalf("Failed to get flags: %v", err)
	}

	if len(flags) != numFlags {
		t.Errorf("Expected %d flags, got %d", numFlags, len(flags))
	}
}

func TestConcurrent_SnapshotReads(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "admin-key")
	handler := srv.Router()
	ctx := context.Background()

	// Seed with some flags
	for i := 0; i < 10; i++ {
		st.UpsertFlag(ctx, store.UpsertParams{
			Key:     fmt.Sprintf("read_test_%d", i),
			Enabled: true,
			Rollout: 100,
			Env:     "prod",
		})
	}
	srv.RebuildSnapshot(ctx, "prod")

	var wg sync.WaitGroup
	numReaders := 100

	// Multiple concurrent reads
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()

			req := httptest.NewRequest(http.MethodGet, "/v1/flags/snapshot", nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("Reader %d got status %d", n, rr.Code)
				return
			}

			var snap snapshot.Snapshot
			if err := json.NewDecoder(rr.Body).Decode(&snap); err != nil {
				t.Errorf("Reader %d failed to decode: %v", n, err)
			}
		}(i)
	}

	wg.Wait()
}

func TestConcurrent_ReadsDuringUpdates(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "admin-key")
	handler := srv.Router()
	ctx := context.Background()

	srv.RebuildSnapshot(ctx, "prod")

	var wg sync.WaitGroup
	numUpdates := 20
	numReads := 50

	// Concurrent updates
	for i := 0; i < numUpdates; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()

			body := fmt.Sprintf(`{
				"key": "concurrent_%d",
				"enabled": %v,
				"rollout": %d
			}`, n, n%2 == 0, (n%100)+1)

			req := httptest.NewRequest(http.MethodPost, "/v1/flags", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer admin-key")
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < numReads; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()

			req := httptest.NewRequest(http.MethodGet, "/v1/flags/snapshot", nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("Read %d failed with status %d", n, rr.Code)
			}
		}(i)
	}

	wg.Wait()

	// Verify final state is consistent
	snap := snapshot.Load()
	if snap == nil {
		t.Error("Final snapshot is nil")
	}
}

func TestConcurrent_SSESubscriptions(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "admin-key")
	ctx := context.Background()
	srv.RebuildSnapshot(ctx, "prod")

	handler := srv.Router()
	numClients := 10

	var wg sync.WaitGroup
	cancels := make([]context.CancelFunc, numClients)

	// Start multiple SSE clients concurrently
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()

			reqCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			cancels[n] = cancel

			req := httptest.NewRequest(http.MethodGet, "/v1/flags/stream", nil)
			req = req.WithContext(reqCtx)

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
		}(i)
	}

	// Wait for clients to connect
	time.Sleep(100 * time.Millisecond)

	// Trigger some updates while clients are connected
	for i := 0; i < 5; i++ {
		st.UpsertFlag(ctx, store.UpsertParams{
			Key:     fmt.Sprintf("sse_concurrent_%d", i),
			Enabled: true,
			Rollout: 100,
			Env:     "prod",
		})
		srv.RebuildSnapshot(ctx, "prod")
		time.Sleep(50 * time.Millisecond)
	}

	// Cancel all clients
	for _, cancel := range cancels {
		if cancel != nil {
			cancel()
		}
	}

	wg.Wait()
}

func TestConcurrent_SameFlag_MultipleUpdates(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "admin-key")
	handler := srv.Router()
	ctx := context.Background()

	srv.RebuildSnapshot(ctx, "prod")

	var wg sync.WaitGroup
	numUpdates := 50

	// Multiple goroutines updating the same flag
	for i := 0; i < numUpdates; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()

			body := fmt.Sprintf(`{
				"key": "shared_flag",
				"enabled": %v,
				"rollout": %d,
				"description": "Update %d"
			}`, n%2 == 0, (n%100)+1, n)

			req := httptest.NewRequest(http.MethodPost, "/v1/flags", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer admin-key")
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("Update %d failed with status %d", n, rr.Code)
			}
		}(i)
	}

	wg.Wait()

	// Verify flag exists and has valid state
	flag, err := st.GetFlagByKey(ctx, "shared_flag")
	if err != nil {
		t.Fatalf("Failed to get shared_flag: %v", err)
	}

	if flag.Key != "shared_flag" {
		t.Errorf("Expected key 'shared_flag', got %s", flag.Key)
	}

	// Rollout should be valid (1-101, since we do (n%100)+1)
	if flag.Rollout < 1 || flag.Rollout > 101 {
		t.Errorf("Invalid rollout value: %d", flag.Rollout)
	}
}

func TestConcurrent_DeleteDuringReads(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "admin-key")
	handler := srv.Router()
	ctx := context.Background()

	// Create initial flags
	for i := 0; i < 20; i++ {
		st.UpsertFlag(ctx, store.UpsertParams{
			Key:     fmt.Sprintf("delete_test_%d", i),
			Enabled: true,
			Rollout: 100,
			Env:     "prod",
		})
	}
	srv.RebuildSnapshot(ctx, "prod")

	var wg sync.WaitGroup

	// Concurrent deletes
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()

			url := fmt.Sprintf("/v1/flags?key=delete_test_%d&env=prod", n)
			req := httptest.NewRequest(http.MethodDelete, url, nil)
			req.Header.Set("Authorization", "Bearer admin-key")
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			req := httptest.NewRequest(http.MethodGet, "/v1/flags/snapshot", nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("Snapshot read failed with status %d", rr.Code)
			}
		}()
	}

	wg.Wait()

	// Verify remaining flags
	flags, err := st.GetAllFlags(ctx, "prod")
	if err != nil {
		t.Fatalf("Failed to get remaining flags: %v", err)
	}

	// Should have 10 flags left (20 - 10 deleted)
	if len(flags) != 10 {
		t.Errorf("Expected 10 remaining flags, got %d", len(flags))
	}
}

func TestConcurrent_ETagConsistency(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "admin-key")
	handler := srv.Router()
	ctx := context.Background()

	// Create initial state
	st.UpsertFlag(ctx, store.UpsertParams{
		Key:     "etag_test",
		Enabled: true,
		Rollout: 100,
		Env:     "prod",
	})
	srv.RebuildSnapshot(ctx, "prod")

	var wg sync.WaitGroup
	numReaders := 100
	etags := make(chan string, numReaders)

	// Many concurrent reads at the same time
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			req := httptest.NewRequest(http.MethodGet, "/v1/flags/snapshot", nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			etag := rr.Header().Get("ETag")
			etags <- etag
		}()
	}

	wg.Wait()
	close(etags)

	// All ETags should be identical since no updates occurred
	var firstETag string
	for etag := range etags {
		if firstETag == "" {
			firstETag = etag
		} else if etag != firstETag {
			t.Errorf("ETag mismatch: expected %s, got %s", firstETag, etag)
		}
	}
}
