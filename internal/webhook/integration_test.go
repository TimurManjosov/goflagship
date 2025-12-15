package webhook

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	dbgen "github.com/TimurManjosov/goflagship/internal/db/gen"
	"github.com/jackc/pgx/v5/pgtype"
)

// TestWebhookIntegration tests webhook delivery with a mock HTTP server
func TestWebhookIntegration(t *testing.T) {
	// Create a channel to collect received webhooks
	received := make(chan Event, 10)
	
	// Create mock webhook server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify headers
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type: application/json, got %s", r.Header.Get("Content-Type"))
		}
		
		signature := r.Header.Get("X-Flagship-Signature")
		if signature == "" {
			t.Error("Missing X-Flagship-Signature header")
		}
		
		eventType := r.Header.Get("X-Flagship-Event")
		if eventType == "" {
			t.Error("Missing X-Flagship-Event header")
		}
		
		deliveryID := r.Header.Get("X-Flagship-Delivery")
		if deliveryID == "" {
			t.Error("Missing X-Flagship-Delivery header")
		}
		
		// Read and decode payload
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("Failed to read request body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		
		var event Event
		if err := json.Unmarshal(body, &event); err != nil {
			t.Errorf("Failed to unmarshal event: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		
		// Verify signature
		secret := "test-secret-123"
		if !VerifySignature(body, signature, secret) {
			t.Error("Signature verification failed")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		
		// Send event to channel
		received <- event
		
		// Respond with success
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer mockServer.Close()

	// Create mock queries that track deliveries
	mockQueries := &mockQueries{
		webhooks: []dbgen.Webhook{
			{
				ID:             uuidFromString("550e8400-e29b-41d4-a716-446655440000"),
				Url:            mockServer.URL,
				Enabled:        true,
				Events:         []string{EventFlagUpdated},
				Secret:         "test-secret-123",
				MaxRetries:     3,
				TimeoutSeconds: 10,
			},
		},
	}

	// Create dispatcher
	dispatcher := NewDispatcher(mockQueries)
	dispatcher.Start()
	defer dispatcher.Stop()

	// Dispatch test event
	testEvent := Event{
		Type:        EventFlagUpdated,
		Timestamp:   time.Now(),
		Environment: "prod",
		Resource: Resource{
			Type: "flag",
			Key:  "test_flag",
		},
		Data: EventData{
			Before: map[string]any{"enabled": false},
			After:  map[string]any{"enabled": true},
			Changes: map[string]any{
				"enabled": map[string]any{
					"before": false,
					"after":  true,
				},
			},
		},
		Metadata: Metadata{
			RequestID: "test-request-123",
		},
	}

	dispatcher.Dispatch(testEvent)

	// Wait for webhook to be received (with timeout)
	select {
	case receivedEvent := <-received:
		// Verify event contents
		if receivedEvent.Type != testEvent.Type {
			t.Errorf("Event type mismatch: got %s, want %s", receivedEvent.Type, testEvent.Type)
		}
		if receivedEvent.Resource.Key != testEvent.Resource.Key {
			t.Errorf("Resource key mismatch: got %s, want %s", receivedEvent.Resource.Key, testEvent.Resource.Key)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for webhook delivery")
	}

	// Give a small delay for the delivery to be logged
	time.Sleep(100 * time.Millisecond)

	// Verify delivery was logged
	mockQueries.mu.Lock()
	deliveryCount := len(mockQueries.deliveries)
	mockQueries.mu.Unlock()

	if deliveryCount == 0 {
		t.Error("Expected delivery to be logged")
	} else {
		delivery := mockQueries.deliveries[0]
		if !delivery.Success {
			t.Error("Expected delivery to be successful")
		}
		if delivery.RetryCount != 0 {
			t.Errorf("Expected retry count to be 0, got %d", delivery.RetryCount)
		}
	}
}

// TestWebhookRetry tests retry logic with failures
func TestWebhookRetry(t *testing.T) {
	attempts := 0
	var mu sync.Mutex

	// Create mock server that fails first 2 times then succeeds
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		attempts++
		currentAttempt := attempts
		mu.Unlock()

		if currentAttempt < 3 {
			// Fail first 2 attempts
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Succeed on 3rd attempt
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	mockQueries := &mockQueries{
		webhooks: []dbgen.Webhook{
			{
				ID:             uuidFromString("550e8400-e29b-41d4-a716-446655440000"),
				Url:            mockServer.URL,
				Enabled:        true,
				Events:         []string{EventFlagCreated},
				Secret:         "test-secret",
				MaxRetries:     3,
				TimeoutSeconds: 5,
			},
		},
	}

	dispatcher := NewDispatcher(mockQueries)
	dispatcher.Start()
	defer dispatcher.Stop()

	testEvent := Event{
		Type:        EventFlagCreated,
		Environment: "prod",
		Resource:    Resource{Type: "flag", Key: "new_flag"},
		Timestamp:   time.Now(),
	}

	dispatcher.Dispatch(testEvent)

	// Wait for retries to complete
	time.Sleep(10 * time.Second)

	mu.Lock()
	finalAttempts := attempts
	mu.Unlock()

	// Should have made 3 attempts (initial + 2 retries before success)
	if finalAttempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", finalAttempts)
	}
}

// mockQueries implements a minimal version of dbgen.Queries for testing
type mockQueries struct {
	webhooks   []dbgen.Webhook
	deliveries []dbgen.CreateWebhookDeliveryParams
	mu         sync.Mutex
}

func (m *mockQueries) GetActiveWebhooks(ctx context.Context) ([]dbgen.Webhook, error) {
	return m.webhooks, nil
}

func (m *mockQueries) UpdateWebhookLastTriggered(ctx context.Context, id pgtype.UUID) error {
	return nil
}

func (m *mockQueries) CreateWebhookDelivery(ctx context.Context, params dbgen.CreateWebhookDeliveryParams) (dbgen.WebhookDelivery, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.deliveries = append(m.deliveries, params)

	return dbgen.WebhookDelivery{
		ID:         uuidFromString("delivery-123"),
		WebhookID:  params.WebhookID,
		EventType:  params.EventType,
		Success:    params.Success,
		RetryCount: params.RetryCount,
	}, nil
}

// Helper to create UUID from string
func uuidFromString(s string) pgtype.UUID {
	var uuid pgtype.UUID
	uuid.Scan(s)
	return uuid
}
