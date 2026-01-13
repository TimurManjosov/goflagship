// Package webhook provides event dispatching and delivery for webhooks.
//
// Webhook Dispatch Flow:
//  1. API handler creates an Event and calls dispatcher.Dispatch(event)
//  2. Event is queued in a buffered channel (non-blocking, async)
//  3. Background worker processes events from queue
//  4. For each event, worker finds matching webhooks (filters by event type and environment)
//  5. Worker attempts delivery to each matching webhook with retry logic
//  6. Delivery attempts are logged to database (webhook_deliveries table)
//  7. Successful deliveries update webhook's last_triggered timestamp
//
// Retry Logic:
//   - Exponential backoff: 1s, 2s, 4s, 8s, etc.
//   - Max retries configured per webhook (default 3)
//   - Permanent failures are logged but don't block processing
//
// How to add a new webhook event type:
//  1. Add event constant to this file (e.g., EventFlagDeleted = "flag.deleted")
//  2. Dispatch event from appropriate API handler: dispatcher.Dispatch(Event{...})
//  3. Update webhook filtering logic in dispatcher.go if needed (usually automatic)
//  4. Document new event type in WEBHOOKS.md
//  5. Add test case in dispatcher_test.go
//
// Thread Safety:
//   - Dispatcher uses a goroutine worker to process events asynchronously
//   - Dispatch() is non-blocking and safe to call from any goroutine
//   - Queue has fixed size (1000); if full, events are dropped with warning
package webhook

import (
	"time"
)

// Event types that can trigger webhooks
const (
	EventFlagCreated = "flag.created"
	EventFlagUpdated = "flag.updated"
	EventFlagDeleted = "flag.deleted"
)

// Event represents a webhook event that will be sent to subscribed webhooks
type Event struct {
	Type        string            `json:"event"`
	Timestamp   time.Time         `json:"timestamp"`
	Project     string            `json:"project,omitempty"`
	Environment string            `json:"environment"`
	Resource    Resource          `json:"resource"`
	Data        EventData         `json:"data"`
	Metadata    Metadata          `json:"metadata"`
}

// Resource identifies the resource that triggered the event
type Resource struct {
	Type string `json:"type"` // e.g., "flag"
	Key  string `json:"key"`  // e.g., flag key
}

// EventData contains the before/after state and changes
type EventData struct {
	Before  map[string]any `json:"before,omitempty"`
	After   map[string]any `json:"after,omitempty"`
	Changes map[string]any `json:"changes,omitempty"`
}

// Metadata contains additional context about the event
type Metadata struct {
	APIKeyID  string `json:"apiKeyId,omitempty"`
	IPAddress string `json:"ipAddress,omitempty"`
	RequestID string `json:"requestId,omitempty"`
}
