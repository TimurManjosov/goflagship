package webhook

import (
	"fmt"
	"net/http"
	"time"

	"github.com/TimurManjosov/goflagship/internal/auth"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgtype"
)

// EventBuilder provides a fluent API for constructing webhook events.
// It simplifies event creation by determining event type automatically and providing defaults.
//
// Usage:
//
//	event := webhook.NewEventBuilder(r).
//		ForFlag(flagKey, env).
//		WithStates(beforeState, afterState).
//		WithChanges(changes).
//		Build()
//	
//	dispatcher.Dispatch(event)
type EventBuilder struct {
	event Event
}

// NewEventBuilder creates a new builder initialized with request context.
// It automatically extracts request ID, IP address, and API key from the HTTP request.
func NewEventBuilder(r *http.Request) *EventBuilder {
	metadata := Metadata{
		RequestID: middleware.GetReqID(r.Context()),
		IPAddress: auth.GetIPAddress(r),
	}
	
	if apiKeyID, ok := auth.GetAPIKeyIDFromContext(r.Context()); ok && apiKeyID.Valid {
		metadata.APIKeyID = formatUUID(apiKeyID)
	}

	return &EventBuilder{
		event: Event{
			Timestamp: time.Now(),
			Metadata:  metadata,
		},
	}
}

// ForFlag sets the resource to a flag with the given key and environment.
func (b *EventBuilder) ForFlag(key, env string) *EventBuilder {
	b.event.Resource = Resource{
		Type: "flag",
		Key:  key,
	}
	b.event.Environment = env
	return b
}

// WithStates sets the before and after states for the event.
// The event type (created/updated/deleted) is automatically determined:
//   - before=nil, after!=nil → created
//   - before!=nil, after=nil → deleted
//   - both non-nil → updated
//   - both nil → no event type set (caller should set explicitly if needed)
func (b *EventBuilder) WithStates(before, after map[string]any) *EventBuilder {
	b.event.Data.Before = before
	b.event.Data.After = after
	
	// Automatically determine event type based on states
	if before == nil && after != nil {
		b.event.Type = EventFlagCreated
	} else if before != nil && after == nil {
		b.event.Type = EventFlagDeleted
	} else if before != nil && after != nil {
		b.event.Type = EventFlagUpdated
	}
	// If both are nil, don't set any event type - let caller handle this edge case
	
	return b
}

// WithChanges sets the changes for the event.
func (b *EventBuilder) WithChanges(changes map[string]any) *EventBuilder {
	b.event.Data.Changes = changes
	return b
}

// Build returns the constructed Event.
// The returned event is ready to be dispatched via dispatcher.Dispatch().
func (b *EventBuilder) Build() Event {
	return b.event
}

// formatUUID formats a pgtype.UUID to string.
// Note: This is duplicated in api/helpers.go and audit/service.go to avoid import cycles.
// Uses fmt.Sprintf for consistency with other packages.
func formatUUID(uuid pgtype.UUID) string {
	if !uuid.Valid {
		return ""
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		uuid.Bytes[0:4], uuid.Bytes[4:6], uuid.Bytes[6:8], uuid.Bytes[8:10], uuid.Bytes[10:16])
}
