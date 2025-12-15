package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
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

// Dispatcher manages webhook event dispatching and delivery
type Dispatcher struct {
	queries *dbgen.Queries
	client  *http.Client
	queue   chan Event
	done    chan struct{}
}

// NewDispatcher creates a new webhook dispatcher
func NewDispatcher(queries *dbgen.Queries) *Dispatcher {
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

// Stop stops the dispatcher and waits for pending events to be processed
func (d *Dispatcher) Stop() {
	close(d.queue)
	<-d.done
}

// Dispatch queues an event for webhook delivery
// This is non-blocking and will not slow down the caller
func (d *Dispatcher) Dispatch(event Event) {
	select {
	case d.queue <- event:
		// Event queued successfully
	default:
		// Queue is full, drop event (in production, we might log this)
		// This ensures we never block the caller
	}
}

// worker processes events from the queue
func (d *Dispatcher) worker() {
	defer close(d.done)
	
	for event := range d.queue {
		webhooks, err := d.getMatchingWebhooks(context.Background(), event)
		if err != nil {
			// Log error but continue processing
			continue
		}

		for _, webhook := range webhooks {
			// Deliver to each matching webhook
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
		d.logDelivery(ctx, webhook.ID, event.Type, payload, 0, "", err.Error(), 0, false, 0)
		return
	}

	signature := ComputeHMAC(payload, webhook.Secret)
	deliveryID := uuid.New().String()

	for attempt := 0; attempt <= int(webhook.MaxRetries); attempt++ {
		start := time.Now()

		req, err := http.NewRequest("POST", webhook.Url, bytes.NewReader(payload))
		if err != nil {
			d.logDelivery(ctx, webhook.ID, event.Type, payload, 0, "", err.Error(), 0, false, attempt)
			return
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Flagship-Signature", signature)
		req.Header.Set("X-Flagship-Event", event.Type)
		req.Header.Set("X-Flagship-Delivery", deliveryID)

		// Create context with timeout for this request
		reqCtx, cancel := context.WithTimeout(ctx, time.Duration(webhook.TimeoutSeconds)*time.Second)
		defer cancel()

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

		success := (err == nil && statusCode >= 200 && statusCode < 300)

		// Log this delivery attempt
		d.logDelivery(ctx, webhook.ID, event.Type, payload, statusCode, responseBody, errorMsg, int(duration.Milliseconds()), success, attempt)

		if success {
			// Update last triggered timestamp
			_ = d.queries.UpdateWebhookLastTriggered(ctx, webhook.ID)
			return // Success, no retry needed
		}

		// Exponential backoff before retry
		if attempt < int(webhook.MaxRetries) {
			backoffDuration := time.Duration(math.Pow(2, float64(attempt))) * time.Second
			time.Sleep(backoffDuration)
		}
	}
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
