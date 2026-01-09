package webhook

import (
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
func (b *EventBuilder) WithStates(before, after map[string]any) *EventBuilder {
	b.event.Data.Before = before
	b.event.Data.After = after
	
	// Automatically determine event type based on states
	if before == nil && after != nil {
		b.event.Type = EventFlagCreated
	} else if before != nil && after == nil {
		b.event.Type = EventFlagDeleted
	} else {
		b.event.Type = EventFlagUpdated
	}
	
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
// This is a helper to avoid import cycles with the api package.
func formatUUID(uuid pgtype.UUID) string {
	if !uuid.Valid {
		return ""
	}
	// Format as standard UUID string: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	return formatUUIDBytes(uuid.Bytes)
}

// formatUUIDBytes formats UUID bytes to string.
func formatUUIDBytes(bytes [16]byte) string {
	return string([]byte{
		hexChars[bytes[0]>>4], hexChars[bytes[0]&0xf],
		hexChars[bytes[1]>>4], hexChars[bytes[1]&0xf],
		hexChars[bytes[2]>>4], hexChars[bytes[2]&0xf],
		hexChars[bytes[3]>>4], hexChars[bytes[3]&0xf],
		'-',
		hexChars[bytes[4]>>4], hexChars[bytes[4]&0xf],
		hexChars[bytes[5]>>4], hexChars[bytes[5]&0xf],
		'-',
		hexChars[bytes[6]>>4], hexChars[bytes[6]&0xf],
		hexChars[bytes[7]>>4], hexChars[bytes[7]&0xf],
		'-',
		hexChars[bytes[8]>>4], hexChars[bytes[8]&0xf],
		hexChars[bytes[9]>>4], hexChars[bytes[9]&0xf],
		'-',
		hexChars[bytes[10]>>4], hexChars[bytes[10]&0xf],
		hexChars[bytes[11]>>4], hexChars[bytes[11]&0xf],
		hexChars[bytes[12]>>4], hexChars[bytes[12]&0xf],
		hexChars[bytes[13]>>4], hexChars[bytes[13]&0xf],
		hexChars[bytes[14]>>4], hexChars[bytes[14]&0xf],
		hexChars[bytes[15]>>4], hexChars[bytes[15]&0xf],
	})
}

var hexChars = []byte("0123456789abcdef")
