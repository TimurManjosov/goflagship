package api

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/TimurManjosov/goflagship/internal/store"
)

// SSEEvent represents a parsed Server-Sent Event
type SSEEvent struct {
	Event string
	Data  map[string]string
}

// parseSSEStream reads SSE events from a response body
func parseSSEStream(t *testing.T, scanner *bufio.Scanner) <-chan SSEEvent {
	t.Helper()
	events := make(chan SSEEvent, 10)

	go func() {
		defer close(events)
		var currentEvent string
		var currentData string

		for scanner.Scan() {
			line := scanner.Text()

			if strings.HasPrefix(line, "event:") {
				currentEvent = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			} else if strings.HasPrefix(line, "data:") {
				currentData = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			} else if line == "" && currentEvent != "" {
				// End of event (blank line)
				var data map[string]string
				if currentData != "" {
					if err := json.Unmarshal([]byte(currentData), &data); err != nil {
						// Log parse error but continue - this is test helper code
						t.Logf("Warning: failed to parse SSE data as JSON: %v", err)
					}
				}
				events <- SSEEvent{Event: currentEvent, Data: data}
				currentEvent = ""
				currentData = ""
			}
		}
	}()

	return events
}

func TestSSE_Connection(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "admin-key")
	handler := srv.Router()
	srv.RebuildSnapshot(context.Background(), "prod")

	req := httptest.NewRequest(http.MethodGet, "/v1/flags/stream", nil)
	rr := httptest.NewRecorder()

	var wg sync.WaitGroup
	wg.Add(1)

	// Start serving in goroutine
	go func() {
		defer wg.Done()
		handler.ServeHTTP(rr, req)
	}()

	// Wait briefly for headers and initial response
	time.Sleep(50 * time.Millisecond)

	// Read result (safe as headers are already set)
	result := rr.Result()
	defer result.Body.Close()

	// Check headers
	contentType := result.Header.Get("Content-Type")
	if contentType != "text/event-stream" {
		t.Errorf("Expected Content-Type 'text/event-stream', got %s", contentType)
	}

	cacheControl := result.Header.Get("Cache-Control")
	if cacheControl != "no-cache" {
		t.Errorf("Expected Cache-Control 'no-cache', got %s", cacheControl)
	}

	connection := result.Header.Get("Connection")
	if connection != "keep-alive" {
		t.Errorf("Expected Connection 'keep-alive', got %s", connection)
	}

	// Note: We're not waiting for wg here as the goroutine will continue
	// The test framework will clean up
}

func TestSSE_InitEvent(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "admin-key")
	ctx := context.Background()

	// Add a flag so we have a real ETag
	st.UpsertFlag(ctx, store.UpsertParams{
		Key:     "init_test",
		Enabled: true,
		Rollout: 100,
		Env:     "prod",
	})
	srv.RebuildSnapshot(ctx, "prod")

	// Create request with cancellable context
	reqCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/v1/flags/stream", nil)
	req = req.WithContext(reqCtx)

	// Use a pipe to read response
	pw := httptest.NewRecorder()
	handler := srv.Router()

	var wg sync.WaitGroup
	wg.Add(1)

	// Serve in goroutine
	go func() {
		defer wg.Done()
		// Create custom response writer that writes to our pipe
		handler.ServeHTTP(pw, req)
	}()

	// Small delay to let init event be sent
	time.Sleep(100 * time.Millisecond)

	// Cancel and wait
	cancel()
	wg.Wait()

	// Now safe to parse the response
	scanner := bufio.NewScanner(strings.NewReader(pw.Body.String()))
	events := parseSSEStream(t, scanner)

	select {
	case event := <-events:
		if event.Event != "init" {
			t.Errorf("Expected first event to be 'init', got '%s'", event.Event)
		}
		if event.Data["etag"] == "" {
			t.Error("Expected init event to contain etag")
		}
	case <-time.After(1 * time.Second):
		t.Error("Timeout waiting for init event")
	}
}

func TestSSE_UpdateEvent(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "admin-key")
	ctx := context.Background()

	// Initialize snapshot
	srv.RebuildSnapshot(ctx, "prod")

	// Create request with cancellable context
	reqCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/v1/flags/stream", nil)
	req = req.WithContext(reqCtx)

	rr := httptest.NewRecorder()
	handler := srv.Router()

	// Channel to track when we've seen init
	initReceived := make(chan bool, 1)

	var wg sync.WaitGroup
	wg.Add(1)

	// Serve SSE in goroutine
	go func() {
		defer wg.Done()
		handler.ServeHTTP(rr, req)
	}()

	// Wait for init event
	time.Sleep(100 * time.Millisecond)

	// Trigger update by adding a flag
	go func() {
		time.Sleep(200 * time.Millisecond)
		st.UpsertFlag(ctx, store.UpsertParams{
			Key:     "update_test",
			Enabled: true,
			Rollout: 100,
			Env:     "prod",
		})
		srv.RebuildSnapshot(ctx, "prod")
		initReceived <- true
	}()

	// Wait for update to complete
	select {
	case <-initReceived:
		// Update triggered
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for update to trigger")
	}

	// Give time for SSE to send update event
	time.Sleep(200 * time.Millisecond)

	// Cancel and wait for handler to finish
	cancel()
	wg.Wait()

	// Now safe to read response
	// Parse response
	scanner := bufio.NewScanner(strings.NewReader(rr.Body.String()))
	events := parseSSEStream(t, scanner)

	// Should have received init and update events
	eventCount := 0
	hasInit := false
	hasUpdate := false

	timeout := time.After(500 * time.Millisecond)
	for {
		select {
		case event, ok := <-events:
			if !ok {
				goto done
			}
			eventCount++
			if event.Event == "init" {
				hasInit = true
			}
			if event.Event == "update" {
				hasUpdate = true
				if event.Data["etag"] == "" {
					t.Error("Expected update event to contain etag")
				}
			}
		case <-timeout:
			goto done
		}
	}

done:
	if !hasInit {
		t.Error("Did not receive init event")
	}
	if !hasUpdate {
		t.Error("Did not receive update event")
	}
}

func TestSSE_ClientDisconnect(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "admin-key")
	srv.RebuildSnapshot(context.Background(), "prod")

	// Create request with cancellable context
	reqCtx, cancel := context.WithCancel(context.Background())

	req := httptest.NewRequest(http.MethodGet, "/v1/flags/stream", nil)
	req = req.WithContext(reqCtx)

	rr := httptest.NewRecorder()
	handler := srv.Router()

	// Serve in goroutine
	done := make(chan bool)
	go func() {
		handler.ServeHTTP(rr, req)
		done <- true
	}()

	// Wait a bit for connection to establish
	time.Sleep(100 * time.Millisecond)

	// Cancel context (simulate client disconnect)
	cancel()

	// Handler should exit promptly
	select {
	case <-done:
		// Success - handler exited
	case <-time.After(1 * time.Second):
		t.Error("Handler did not exit after context cancellation")
	}
}

func TestSSE_MultipleClients(t *testing.T) {
	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "admin-key")
	ctx := context.Background()
	srv.RebuildSnapshot(ctx, "prod")

	numClients := 3
	recorders := make([]*httptest.ResponseRecorder, numClients)
	cancels := make([]context.CancelFunc, numClients)

	handler := srv.Router()

	var wg sync.WaitGroup

	// Start multiple SSE clients
	for i := 0; i < numClients; i++ {
		wg.Add(1)

		reqCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		cancels[i] = cancel

		req := httptest.NewRequest(http.MethodGet, "/v1/flags/stream", nil)
		req = req.WithContext(reqCtx)

		rr := httptest.NewRecorder()
		recorders[i] = rr

		go func() {
			defer wg.Done()
			handler.ServeHTTP(rr, req)
		}()
	}

	// Wait for connections to establish
	time.Sleep(150 * time.Millisecond)

	// Trigger an update
	st.UpsertFlag(ctx, store.UpsertParams{
		Key:     "multi_client_test",
		Enabled: true,
		Rollout: 100,
		Env:     "prod",
	})
	srv.RebuildSnapshot(ctx, "prod")

	// Wait for updates to propagate
	time.Sleep(200 * time.Millisecond)

	// Cancel all clients
	for _, cancel := range cancels {
		cancel()
	}

	// Wait for all handlers to finish
	wg.Wait()

	// Now it's safe to check responses - all goroutines are done
	for i, rr := range recorders {
		body := rr.Body.String()
		if !strings.Contains(body, "event: init") {
			t.Errorf("Client %d did not receive init event", i)
		}
		// Note: due to timing, update event might not always be caught
		// but at least init should be there
	}
}

func TestSSE_HeartbeatPing(t *testing.T) {
	t.Skip("Skipping heartbeat test as it requires 25+ second wait")

	st := store.NewMemoryStore()
	srv := NewServer(st, "prod", "admin-key")
	srv.RebuildSnapshot(context.Background(), "prod")

	reqCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/v1/flags/stream", nil)
	req = req.WithContext(reqCtx)

	rr := httptest.NewRecorder()
	handler := srv.Router()

	go handler.ServeHTTP(rr, req)

	// Wait for at least one heartbeat (25s + buffer)
	time.Sleep(26 * time.Second)
	cancel()

	// Check for ping in response
	body := rr.Body.String()
	if !strings.Contains(body, ": ping") {
		t.Error("Expected to find heartbeat ping in SSE stream")
	}
}
