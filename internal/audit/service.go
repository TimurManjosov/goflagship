package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgtype"
)

// Action constants for audit logging
const (
	ActionCreated     = "created"
	ActionUpdated     = "updated"
	ActionDeleted     = "deleted"
	ActionAuthFailed  = "auth_failed"
	ActionEvaluated   = "evaluated"
	ActionAccessed    = "accessed"
)

// ResourceType constants for audit logging
const (
	ResourceTypeFlag    = "flag"
	ResourceTypeProject = "project"
	ResourceTypeAPIKey  = "api_key"
	ResourceTypeSystem  = "system"
)

// Status constants for audit logging
const (
	StatusSuccess = "success"
	StatusFailure = "failure"
)

// ActorKind constants for audit logging
const (
	ActorKindAPIKey = "api_key"
	ActorKindUser   = "user"
	ActorKindSystem = "system"
)

// Clock interface for testable time operations
type Clock interface {
	Now() time.Time
}

// SystemClock implements Clock using time.Now()
type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now() }

// IDGenerator interface for testable ID generation
type IDGenerator interface {
	Generate() string
}

// UUIDGenerator implements IDGenerator using UUID v4
type UUIDGenerator struct{}

func (UUIDGenerator) Generate() string {
	// Simple implementation - in production might use google/uuid
	return fmt.Sprintf("req-%d", time.Now().UnixNano())
}

// Redactor interface for removing sensitive data
type Redactor interface {
	Redact(data map[string]any) map[string]any
}

// DefaultRedactor implements basic redaction
type DefaultRedactor struct {
	sensitiveKeys []string
}

func NewDefaultRedactor() *DefaultRedactor {
	return &DefaultRedactor{
		sensitiveKeys: []string{
			"password", "secret", "token", "api_key", "key_hash",
			"authorization", "cookie", "session",
		},
	}
}

func (r *DefaultRedactor) Redact(data map[string]any) map[string]any {
	if data == nil {
		return nil
	}
	
	redacted := make(map[string]any)
	for k, v := range data {
		// Check if key is sensitive
		isSensitive := false
		for _, sensitive := range r.sensitiveKeys {
			if k == sensitive {
				isSensitive = true
				break
			}
		}
		
		if isSensitive {
			redacted[k] = "[REDACTED]"
		} else if nested, ok := v.(map[string]any); ok {
			// Recursively redact nested maps
			redacted[k] = r.Redact(nested)
		} else {
			redacted[k] = v
		}
	}
	return redacted
}

// Actor represents who performed the action
type Actor struct {
	Kind    string  `json:"kind"`    // api_key, user, system
	ID      *string `json:"id,omitempty"`
	Email   *string `json:"email,omitempty"`
	Display string  `json:"display"` // Human-readable identifier
}

// Source represents request metadata
type Source struct {
	IPAddress string `json:"ip_address"`
	UserAgent string `json:"user_agent"`
}

// AuditEvent represents a canonical audit event
type AuditEvent struct {
	OccurredAt   time.Time      `json:"occurred_at"`
	RequestID    string         `json:"request_id"`
	Actor        Actor          `json:"actor"`
	Source       Source         `json:"source"`
	Action       string         `json:"action"`        // created, updated, deleted, etc.
	ResourceType string         `json:"resource_type"` // flag, project, api_key
	ResourceID   string         `json:"resource_id"`
	ProjectID    *string        `json:"project_id,omitempty"`
	Environment  *string        `json:"environment,omitempty"`
	BeforeState  map[string]any `json:"before_state,omitempty"`
	AfterState   map[string]any `json:"after_state,omitempty"`
	Changes      map[string]any `json:"changes,omitempty"`
	Status       string         `json:"status"` // success, failure
	ErrorMessage *string        `json:"error_message,omitempty"`
}

// AuditSink defines the interface for persisting audit events
type AuditSink interface {
	Write(ctx context.Context, event AuditEvent) error
}

// Service provides audit logging functionality
type Service struct {
	sink     AuditSink
	clock    Clock
	idgen    IDGenerator
	redactor Redactor
	queue    chan AuditEvent
	stopCh   chan struct{}
	closed   int32 // atomic flag to prevent double-close
}

// NewService creates a new audit service
func NewService(sink AuditSink, clock Clock, idgen IDGenerator, redactor Redactor, queueSize int) *Service {
	if clock == nil {
		clock = SystemClock{}
	}
	if idgen == nil {
		idgen = UUIDGenerator{}
	}
	if redactor == nil {
		redactor = NewDefaultRedactor()
	}
	
	s := &Service{
		sink:     sink,
		clock:    clock,
		idgen:    idgen,
		redactor: redactor,
		queue:    make(chan AuditEvent, queueSize),
		stopCh:   make(chan struct{}),
	}
	
	// Start background worker
	go s.worker()
	
	return s
}

// worker processes audit events in the background
func (s *Service) worker() {
	for {
		select {
		case event := <-s.queue:
			// Use background context with timeout for persistence
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := s.sink.Write(ctx, event); err != nil {
				// Log error but don't fail - audit logging must be non-blocking
				log.Printf("audit: failed to write event: %v", err)
			}
			cancel()
		case <-s.stopCh:
			// Drain remaining events before stopping
			for len(s.queue) > 0 {
				event := <-s.queue
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				_ = s.sink.Write(ctx, event)
				cancel()
			}
			return
		}
	}
}

// Close gracefully shuts down the audit service.
// It signals the background worker to stop and drains any remaining events in the queue.
// After Close is called, no new events should be logged.
//
// Close is safe to call multiple times - subsequent calls are no-ops.
// Close blocks until all pending events are processed or a timeout is reached.
func (s *Service) Close() error {
	// Atomically check if already closed
	if !atomic.CompareAndSwapInt32(&s.closed, 0, 1) {
		return nil // Already closed
	}
	// Signal worker to stop
	close(s.stopCh)
	// Worker will drain queue and exit
	return nil
}

// Log queues an audit event for asynchronous processing
func (s *Service) Log(event AuditEvent) {
	// Ensure occurred_at is set
	if event.OccurredAt.IsZero() {
		event.OccurredAt = s.clock.Now()
	}
	
	// Ensure request_id is set
	if event.RequestID == "" {
		event.RequestID = s.idgen.Generate()
	}
	
	// Redact sensitive data in states
	if event.BeforeState != nil {
		event.BeforeState = s.redactor.Redact(event.BeforeState)
	}
	if event.AfterState != nil {
		event.AfterState = s.redactor.Redact(event.AfterState)
	}
	
	// Try to queue, drop if full
	select {
	case s.queue <- event:
		// Successfully queued
	default:
		// Queue full - log and drop
		log.Printf("audit: queue full, dropping event for %s/%s", event.ResourceType, event.ResourceID)
	}
}

// LogFromContext is a helper that extracts common fields from context/request
func (s *Service) LogFromContext(ctx context.Context, action, resourceType, resourceID string, status string) {
	event := AuditEvent{
		RequestID:    middleware.GetReqID(ctx),
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Status:       status,
	}
	
	// Extract actor from context if available
	if apiKeyID, ok := ctx.Value("api_key_id").(pgtype.UUID); ok && apiKeyID.Valid {
		idStr := formatUUID(apiKeyID)
		event.Actor = Actor{
			Kind:    ActorKindAPIKey,
			ID:      &idStr,
			Display: fmt.Sprintf("api_key:%s", idStr[:8]),
		}
	} else {
		event.Actor = Actor{
			Kind:    ActorKindSystem,
			Display: "system",
		}
	}
	
	s.Log(event)
}

// Stop gracefully shuts down the audit service
func (s *Service) Stop() {
	close(s.stopCh)
}

// ComputeChanges computes the difference between before and after states
func ComputeChanges(before, after map[string]any) map[string]any {
	if before == nil && after == nil {
		return nil
	}
	if before == nil {
		before = make(map[string]any)
	}
	if after == nil {
		after = make(map[string]any)
	}
	
	changes := make(map[string]any)
	
	// Check for changes in after (new or modified values)
	for key, afterVal := range after {
		beforeVal, existedBefore := before[key]
		
		// Compare values
		beforeJSON, _ := json.Marshal(beforeVal)
		afterJSON, _ := json.Marshal(afterVal)
		
		if !existedBefore || string(beforeJSON) != string(afterJSON) {
			changes[key] = map[string]any{
				"before": beforeVal,
				"after":  afterVal,
			}
		}
	}
	
	// Check for removed keys
	for key, beforeVal := range before {
		if _, existsAfter := after[key]; !existsAfter {
			changes[key] = map[string]any{
				"before": beforeVal,
				"after":  nil,
			}
		}
	}
	
	if len(changes) == 0 {
		return nil
	}
	
	return changes
}

// formatUUID formats a pgtype.UUID to string.
// Note: This is duplicated in api/helpers.go and webhook/builder.go to avoid import cycles.
// All implementations use the same fmt.Sprintf format for consistency.
func formatUUID(uuid pgtype.UUID) string {
	if !uuid.Valid {
		return ""
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		uuid.Bytes[0:4], uuid.Bytes[4:6], uuid.Bytes[6:8], uuid.Bytes[8:10], uuid.Bytes[10:16])
}
