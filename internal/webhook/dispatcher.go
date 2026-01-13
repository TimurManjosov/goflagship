package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"sync/atomic"
	"time"

	dbgen "github.com/TimurManjosov/goflagship/internal/db/gen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	// queueSize is the buffer size for the event queue
	queueSize = 1000

	// maxResponseBodySize limits how much of the response body we store (1KB)
	maxResponseBodySize = 1024
)

// WebhookQueries defines the interface for webhook database operations
type WebhookQueries interface {
	GetActiveWebhooks(ctx context.Context) ([]dbgen.Webhook, error)
	UpdateWebhookLastTriggered(ctx context.Context, id pgtype.UUID) error
	CreateWebhookDelivery(ctx context.Context, params dbgen.CreateWebhookDeliveryParams) (dbgen.WebhookDelivery, error)
}

// Dispatcher manages webhook event dispatching and delivery
type Dispatcher struct {
	queries WebhookQueries
	client  *http.Client
	queue   chan Event
	done    chan struct{}
	closed  int32 // atomic flag to prevent double-close
}

// NewDispatcher creates a new webhook dispatcher
func NewDispatcher(queries WebhookQueries) *Dispatcher {
	return &Dispatcher{
		queries: queries,
		client: &http.Client{
			// Default timeout, will be overridden per-webhook
			Timeout: 10 * time.Second,
		},
		queue: make(chan Event, queueSize),
		done:  make(chan struct{}),
	}
}

// Start begins processing events from the queue
func (d *Dispatcher) Start() {
	go d.worker()
}

// Stop stops the dispatcher and waits for pending events to be processed.
// Deprecated: Use Close() instead for consistent lifecycle management.
func (d *Dispatcher) Stop() {
	_ = d.Close()
}

// Close gracefully shuts down the webhook dispatcher.
// It closes the event queue and waits for all pending deliveries to complete.
// After Close is called, no new events should be dispatched.
//
// Close is safe to call multiple times - subsequent calls are no-ops.
// Close implements the io.Closer interface for consistent resource management.
func (d *Dispatcher) Close() error {
	// Atomically check if already closed
	if !atomic.CompareAndSwapInt32(&d.closed, 0, 1) {
		return nil // Already closed
	}
	close(d.queue)
	<-d.done
	return nil
}

// Dispatch queues an event for webhook delivery
// This is non-blocking and will not slow down the caller
func (d *Dispatcher) Dispatch(event Event) {
	select {
	case d.queue <- event:
		log.Printf("[webhook] event queued: type=%s resource=%s/%s env=%s queue_size=%d",
			event.Type, event.Resource.Type, event.Resource.Key, event.Environment, len(d.queue))
	default:
		// Queue is full, drop event and log warning
		log.Printf("[webhook] CRITICAL: queue full (size=%d), dropping event: type=%s resource=%s/%s env=%s",
			queueSize, event.Type, event.Resource.Type, event.Resource.Key, event.Environment)
		// Note: In production, consider adding a metric here for monitoring
	}
}

// worker processes events from the queue
func (d *Dispatcher) worker() {
	defer close(d.done)
	
	for event := range d.queue {
		log.Printf("[webhook] processing event: type=%s resource=%s/%s env=%s",
			event.Type, event.Resource.Type, event.Resource.Key, event.Environment)
		
		webhooks, err := d.getMatchingWebhooks(context.Background(), event)
		if err != nil {
			log.Printf("[webhook] failed to fetch webhooks for event: type=%s resource=%s/%s env=%s error=%v",
				event.Type, event.Resource.Type, event.Resource.Key, event.Environment, err)
			continue
		}

		log.Printf("[webhook] found %d matching webhook(s) for event: type=%s resource=%s/%s",
			len(webhooks), event.Type, event.Resource.Type, event.Resource.Key)

		for _, webhook := range webhooks {
			d.deliverWithRetry(context.Background(), webhook, event)
		}
	}
}

// getMatchingWebhooks finds all webhooks that should receive this event
func (d *Dispatcher) getMatchingWebhooks(ctx context.Context, event Event) ([]dbgen.Webhook, error) {
	// Get all active webhooks
	allWebhooks, err := d.queries.GetActiveWebhooks(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get active webhooks: %w", err)
	}

	var matching []dbgen.Webhook
	for _, webhook := range allWebhooks {
		if d.matches(webhook, event) {
			matching = append(matching, webhook)
		}
	}

	return matching, nil
}

// matches checks if a webhook should receive this event based on filters
func (d *Dispatcher) matches(webhook dbgen.Webhook, event Event) bool {
	// Check if event type matches
	eventMatches := false
	for _, e := range webhook.Events {
		if e == event.Type {
			eventMatches = true
			break
		}
	}
	if !eventMatches {
		return false
	}

	// Check environment filter (if specified)
	if len(webhook.Environments) > 0 {
		envMatches := false
		for _, env := range webhook.Environments {
			if env == event.Environment {
				envMatches = true
				break
			}
		}
		if !envMatches {
			return false
		}
	}

	// Note: project_id filtering would go here if we had projects
	// For now, we don't filter by project since the schema doesn't have projects yet

	return true
}

// deliverWithRetry attempts to deliver an event to a webhook with retry logic
func (d *Dispatcher) deliverWithRetry(ctx context.Context, webhook dbgen.Webhook, event Event) {
	payload, err := json.Marshal(event)
	if err != nil {
		// This should not happen, but if it does, log delivery failure
		log.Printf("[webhook] failed to marshal event payload: webhook_id=%s event_type=%s error=%v",
			formatWebhookID(webhook.ID), event.Type, err)
		d.logDelivery(ctx, webhook.ID, event.Type, payload, 0, "", err.Error(), 0, false, 0)
		return
	}

	signature := ComputeHMAC(payload, webhook.Secret)
	deliveryID := uuid.New().String()
	webhookIDStr := formatWebhookID(webhook.ID)

	for attempt := 0; attempt <= int(webhook.MaxRetries); attempt++ {
		start := time.Now()

		log.Printf("[webhook] delivering: webhook_id=%s url=%s event_type=%s attempt=%d/%d",
			webhookIDStr, webhook.Url, event.Type, attempt+1, webhook.MaxRetries+1)

		req, err := http.NewRequest("POST", webhook.Url, bytes.NewReader(payload))
		if err != nil {
			log.Printf("[webhook] failed to create request: webhook_id=%s url=%s error=%v",
				webhookIDStr, webhook.Url, err)
			d.logDelivery(ctx, webhook.ID, event.Type, payload, 0, "", err.Error(), 0, false, attempt)
			return
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Flagship-Signature", signature)
		req.Header.Set("X-Flagship-Event", event.Type)
		req.Header.Set("X-Flagship-Delivery", deliveryID)

		// Create context with timeout for this request
		reqCtx, cancel := context.WithTimeout(ctx, time.Duration(webhook.TimeoutSeconds)*time.Second)
		
		resp, err := d.client.Do(req.WithContext(reqCtx))
		duration := time.Since(start)

		var statusCode int
		var responseBody string
		var errorMsg string

		if err != nil {
			errorMsg = err.Error()
		} else {
			statusCode = resp.StatusCode
			// Read response body (limited size)
			bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodySize))
			responseBody = string(bodyBytes)
			resp.Body.Close()
		}

		// Cancel context immediately after request completes
		cancel()

		success := (err == nil && statusCode >= 200 && statusCode < 300)

		// Log this delivery attempt
		d.logDelivery(ctx, webhook.ID, event.Type, payload, statusCode, responseBody, errorMsg, int(duration.Milliseconds()), success, attempt)

		if success {
			log.Printf("[webhook] delivery succeeded: webhook_id=%s status=%d duration=%dms attempt=%d/%d",
				webhookIDStr, statusCode, duration.Milliseconds(), attempt+1, webhook.MaxRetries+1)
			// Update last triggered timestamp
			_ = d.queries.UpdateWebhookLastTriggered(ctx, webhook.ID)
			return // Success, no retry needed
		}

		// Delivery failed
		if attempt < int(webhook.MaxRetries) {
			backoffDuration := time.Duration(math.Pow(2, float64(attempt))) * time.Second
			log.Printf("[webhook] delivery failed: webhook_id=%s status=%d error=%q attempt=%d/%d retry_in=%s",
				webhookIDStr, statusCode, errorMsg, attempt+1, webhook.MaxRetries+1, backoffDuration)
			time.Sleep(backoffDuration)
		} else {
			log.Printf("[webhook] delivery failed permanently: webhook_id=%s status=%d error=%q attempts=%d/%d",
				webhookIDStr, statusCode, errorMsg, attempt+1, webhook.MaxRetries+1)
		}
	}
}

// formatWebhookID converts a UUID to a string for logging
func formatWebhookID(id pgtype.UUID) string {
	if !id.Valid {
		return "unknown"
	}
	// Convert UUID bytes to string
	u, err := uuid.FromBytes(id.Bytes[:])
	if err != nil {
		return "invalid"
	}
	return u.String()
}

// logDelivery records a webhook delivery attempt in the database
func (d *Dispatcher) logDelivery(ctx context.Context, webhookID pgtype.UUID, eventType string, payload []byte, statusCode int, responseBody string, errorMsg string, durationMs int, success bool, retryCount int) {
	params := dbgen.CreateWebhookDeliveryParams{
		WebhookID: webhookID,
		EventType: eventType,
		Payload:   payload,
		Success:   success,
		RetryCount: int32(retryCount),
	}

	if statusCode > 0 {
		params.StatusCode = pgtype.Int4{Int32: int32(statusCode), Valid: true}
	}

	if responseBody != "" {
		params.ResponseBody = pgtype.Text{String: responseBody, Valid: true}
	}

	if errorMsg != "" {
		params.ErrorMessage = pgtype.Text{String: errorMsg, Valid: true}
	}

	if durationMs > 0 {
		params.DurationMs = pgtype.Int4{Int32: int32(durationMs), Valid: true}
	}

	// Log delivery (fire and forget, don't fail the delivery if logging fails)
	_, _ = d.queries.CreateWebhookDelivery(ctx, params)
}
