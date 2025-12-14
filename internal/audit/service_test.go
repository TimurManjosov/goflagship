package audit

import (
	"context"
	"testing"
	"time"
)

// MockSink is a test implementation of AuditSink
type MockSink struct {
	events []AuditEvent
	err    error
}

func (m *MockSink) Write(ctx context.Context, event AuditEvent) error {
	if m.err != nil {
		return m.err
	}
	m.events = append(m.events, event)
	return nil
}

// MockClock is a test implementation of Clock
type MockClock struct {
	now time.Time
}

func (m *MockClock) Now() time.Time {
	return m.now
}

// MockIDGen is a test implementation of IDGenerator
type MockIDGen struct {
	id string
}

func (m *MockIDGen) Generate() string {
	return m.id
}

func TestComputeChanges(t *testing.T) {
	tests := []struct {
		name   string
		before map[string]any
		after  map[string]any
		want   int // number of changes
	}{
		{
			name:   "no changes",
			before: map[string]any{"key": "value"},
			after:  map[string]any{"key": "value"},
			want:   0,
		},
		{
			name:   "value changed",
			before: map[string]any{"key": "old"},
			after:  map[string]any{"key": "new"},
			want:   1,
		},
		{
			name:   "key added",
			before: map[string]any{"key1": "value1"},
			after:  map[string]any{"key1": "value1", "key2": "value2"},
			want:   1,
		},
		{
			name:   "key removed",
			before: map[string]any{"key1": "value1", "key2": "value2"},
			after:  map[string]any{"key1": "value1"},
			want:   1,
		},
		{
			name:   "both nil",
			before: nil,
			after:  nil,
			want:   -1, // should return nil
		},
		{
			name:   "multiple changes",
			before: map[string]any{"a": 1, "b": 2, "c": 3},
			after:  map[string]any{"a": 10, "b": 2, "d": 4},
			want:   3, // a changed, c removed, d added
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changes := ComputeChanges(tt.before, tt.after)
			
			if tt.want == -1 {
				if changes != nil {
					t.Errorf("expected nil, got %v", changes)
				}
				return
			}
			
			if len(changes) != tt.want {
				t.Errorf("expected %d changes, got %d: %v", tt.want, len(changes), changes)
			}
		})
	}
}

func TestRedactor(t *testing.T) {
	redactor := NewDefaultRedactor()
	
	tests := []struct {
		name  string
		input map[string]any
		check func(t *testing.T, output map[string]any)
	}{
		{
			name:  "redacts password",
			input: map[string]any{"password": "secret123", "username": "alice"},
			check: func(t *testing.T, output map[string]any) {
				if output["password"] != "[REDACTED]" {
					t.Errorf("password not redacted: %v", output["password"])
				}
				if output["username"] != "alice" {
					t.Errorf("username should not be redacted: %v", output["username"])
				}
			},
		},
		{
			name:  "redacts api_key",
			input: map[string]any{"api_key": "key_123", "name": "test"},
			check: func(t *testing.T, output map[string]any) {
				if output["api_key"] != "[REDACTED]" {
					t.Errorf("api_key not redacted: %v", output["api_key"])
				}
			},
		},
		{
			name:  "handles nested maps",
			input: map[string]any{"config": map[string]any{"password": "secret", "url": "http://example.com"}},
			check: func(t *testing.T, output map[string]any) {
				config, ok := output["config"].(map[string]any)
				if !ok {
					t.Fatal("config not a map")
				}
				if config["password"] != "[REDACTED]" {
					t.Errorf("nested password not redacted: %v", config["password"])
				}
				if config["url"] != "http://example.com" {
					t.Errorf("nested url should not be redacted")
				}
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := redactor.Redact(tt.input)
			tt.check(t, output)
		})
	}
}

func TestService_Log(t *testing.T) {
	sink := &MockSink{}
	clock := &MockClock{now: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)}
	idgen := &MockIDGen{id: "test-req-123"}
	
	svc := NewService(sink, clock, idgen, NewDefaultRedactor(), 10)
	defer svc.Stop()
	
	// Log an event
	svc.Log(AuditEvent{
		Action:       ActionCreated,
		ResourceType: ResourceTypeFlag,
		ResourceID:   "test-flag",
		Status:       StatusSuccess,
		BeforeState:  nil,
		AfterState:   map[string]any{"enabled": true},
	})
	
	// Give worker time to process
	time.Sleep(100 * time.Millisecond)
	
	if len(sink.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(sink.events))
	}
	
	event := sink.events[0]
	
	if event.Action != ActionCreated {
		t.Errorf("expected action %s, got %s", ActionCreated, event.Action)
	}
	
	if event.ResourceType != ResourceTypeFlag {
		t.Errorf("expected resource type %s, got %s", ResourceTypeFlag, event.ResourceType)
	}
	
	if event.RequestID != "test-req-123" {
		t.Errorf("expected request ID test-req-123, got %s", event.RequestID)
	}
	
	if !event.OccurredAt.Equal(clock.now) {
		t.Errorf("expected occurred_at %v, got %v", clock.now, event.OccurredAt)
	}
}

func TestService_Redaction(t *testing.T) {
	sink := &MockSink{}
	svc := NewService(sink, SystemClock{}, UUIDGenerator{}, NewDefaultRedactor(), 10)
	defer svc.Stop()
	
	// Log an event with sensitive data
	svc.Log(AuditEvent{
		Action:       ActionUpdated,
		ResourceType: ResourceTypeAPIKey,
		ResourceID:   "key-123",
		Status:       StatusSuccess,
		BeforeState:  map[string]any{"key_hash": "secret_hash", "name": "test-key"},
		AfterState:   map[string]any{"key_hash": "new_secret_hash", "name": "test-key-updated"},
	})
	
	// Give worker time to process
	time.Sleep(100 * time.Millisecond)
	
	if len(sink.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(sink.events))
	}
	
	event := sink.events[0]
	
	// Check that key_hash is redacted
	if event.BeforeState["key_hash"] != "[REDACTED]" {
		t.Errorf("before_state key_hash not redacted: %v", event.BeforeState["key_hash"])
	}
	
	if event.AfterState["key_hash"] != "[REDACTED]" {
		t.Errorf("after_state key_hash not redacted: %v", event.AfterState["key_hash"])
	}
	
	// Check that name is not redacted
	if event.BeforeState["name"] != "test-key" {
		t.Errorf("before_state name should not be redacted: %v", event.BeforeState["name"])
	}
}
