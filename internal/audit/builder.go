package audit

import (
	"net/http"

	"github.com/TimurManjosov/goflagship/internal/auth"
	"github.com/go-chi/chi/v5/middleware"
)

// EventBuilder provides a fluent API for constructing audit events.
// It simplifies event creation by providing sensible defaults and method chaining.
//
// Usage:
//
//	event := audit.NewEventBuilder(r).
//		ForResource(audit.ResourceTypeFlag, flagKey).
//		WithAction(audit.ActionCreated).
//		WithEnvironment(env).
//		WithAfterState(flagData).
//		Success().
//		Build()
//	
//	service.Log(event)
type EventBuilder struct {
	event AuditEvent
}

// NewEventBuilder creates a new builder initialized with request context.
// It automatically extracts request ID, actor, and source information from the HTTP request.
func NewEventBuilder(r *http.Request) *EventBuilder {
	// Extract actor from context
	actor := Actor{
		Kind:    ActorKindSystem,
		Display: "system",
	}
	
	if apiKeyID, ok := auth.GetAPIKeyIDFromContext(r.Context()); ok && apiKeyID.Valid {
		idStr := formatUUID(apiKeyID)
		actor = Actor{
			Kind:    ActorKindAPIKey,
			ID:      &idStr,
			Display: "api_key:" + idStr[:8],
		}
	}

	return &EventBuilder{
		event: AuditEvent{
			RequestID: middleware.GetReqID(r.Context()),
			Actor:     actor,
			Source: Source{
				IPAddress: auth.GetIPAddress(r),
				UserAgent: r.UserAgent(),
			},
			Status: StatusSuccess, // Default to success, caller can override
		},
	}
}

// ForResource sets the resource type and ID for the event.
func (b *EventBuilder) ForResource(resourceType, resourceID string) *EventBuilder {
	b.event.ResourceType = resourceType
	b.event.ResourceID = resourceID
	return b
}

// WithAction sets the action for the event (created, updated, deleted, etc.).
func (b *EventBuilder) WithAction(action string) *EventBuilder {
	b.event.Action = action
	return b
}

// WithEnvironment sets the environment for the event.
func (b *EventBuilder) WithEnvironment(env string) *EventBuilder {
	if env != "" {
		b.event.Environment = &env
	}
	return b
}

// WithBeforeState sets the before state for the event.
func (b *EventBuilder) WithBeforeState(state map[string]any) *EventBuilder {
	if state != nil {
		b.event.BeforeState = state
	}
	return b
}

// WithAfterState sets the after state for the event.
func (b *EventBuilder) WithAfterState(state map[string]any) *EventBuilder {
	if state != nil {
		b.event.AfterState = state
	}
	return b
}

// WithChanges sets the changes for the event.
func (b *EventBuilder) WithChanges(changes map[string]any) *EventBuilder {
	if changes != nil {
		b.event.Changes = changes
	}
	return b
}

// Success marks the event as successful (default).
func (b *EventBuilder) Success() *EventBuilder {
	b.event.Status = StatusSuccess
	return b
}

// Failure marks the event as failed and sets an error message.
func (b *EventBuilder) Failure(errorMsg string) *EventBuilder {
	b.event.Status = StatusFailure
	if errorMsg != "" {
		b.event.ErrorMessage = &errorMsg
	}
	return b
}

// Build returns the constructed AuditEvent.
// The returned event is ready to be logged via service.Log().
func (b *EventBuilder) Build() AuditEvent {
	return b.event
}
