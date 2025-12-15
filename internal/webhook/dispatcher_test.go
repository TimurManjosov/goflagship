package webhook

import (
	"encoding/json"
	"testing"

	dbgen "github.com/TimurManjosov/goflagship/internal/db/gen"
)

func TestDispatcher_matches(t *testing.T) {
	d := &Dispatcher{}

	tests := []struct {
		name    string
		webhook dbgen.Webhook
		event   Event
		want    bool
	}{
		{
			name: "matches event type",
			webhook: dbgen.Webhook{
				Events: []string{EventFlagCreated, EventFlagUpdated},
			},
			event: Event{
				Type: EventFlagUpdated,
			},
			want: true,
		},
		{
			name: "does not match event type",
			webhook: dbgen.Webhook{
				Events: []string{EventFlagCreated},
			},
			event: Event{
				Type: EventFlagDeleted,
			},
			want: false,
		},
		{
			name: "matches environment filter",
			webhook: dbgen.Webhook{
				Events:       []string{EventFlagUpdated},
				Environments: []string{"prod", "staging"},
			},
			event: Event{
				Type:        EventFlagUpdated,
				Environment: "prod",
			},
			want: true,
		},
		{
			name: "does not match environment filter",
			webhook: dbgen.Webhook{
				Events:       []string{EventFlagUpdated},
				Environments: []string{"prod"},
			},
			event: Event{
				Type:        EventFlagUpdated,
				Environment: "dev",
			},
			want: false,
		},
		{
			name: "no environment filter matches all",
			webhook: dbgen.Webhook{
				Events:       []string{EventFlagUpdated},
				Environments: []string{},
			},
			event: Event{
				Type:        EventFlagUpdated,
				Environment: "any-env",
			},
			want: true,
		},
		{
			name: "multiple event types",
			webhook: dbgen.Webhook{
				Events: []string{EventFlagCreated, EventFlagUpdated, EventFlagDeleted},
			},
			event: Event{
				Type: EventFlagDeleted,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := d.matches(tt.webhook, tt.event)
			if got != tt.want {
				t.Errorf("matches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvent_JSONMarshaling(t *testing.T) {
	event := Event{
		Type:        EventFlagUpdated,
		Environment: "prod",
		Resource: Resource{
			Type: "flag",
			Key:  "feature_x",
		},
		Data: EventData{
			Before: map[string]any{
				"enabled": true,
				"rollout": 50,
			},
			After: map[string]any{
				"enabled": false,
				"rollout": 50,
			},
			Changes: map[string]any{
				"enabled": map[string]any{
					"before": true,
					"after":  false,
				},
			},
		},
		Metadata: Metadata{
			APIKeyID:  "key-123",
			IPAddress: "192.168.1.100",
			RequestID: "req-456",
		},
	}

	// Marshal to JSON using standard json.Marshal
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Failed to marshal event: %v", err)
	}

	// Check that marshaled data is not empty
	if len(data) == 0 {
		t.Errorf("Marshaled event is empty")
	}

	// Unmarshal back using standard json.Unmarshal
	var decoded Event
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal event: %v", err)
	}

	// Check key fields
	if decoded.Type != event.Type {
		t.Errorf("Event type mismatch: got %v, want %v", decoded.Type, event.Type)
	}
	if decoded.Environment != event.Environment {
		t.Errorf("Environment mismatch: got %v, want %v", decoded.Environment, event.Environment)
	}
}
