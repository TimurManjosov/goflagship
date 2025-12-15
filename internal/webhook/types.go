package webhook

import (
	"encoding/json"
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

// MarshalJSON implements json.Marshaler
func (e Event) MarshalJSON() ([]byte, error) {
	type Alias Event
	return json.Marshal(&struct {
		Alias
	}{
		Alias: (Alias)(e),
	})
}

// UnmarshalJSON implements json.Unmarshaler
func (e *Event) UnmarshalJSON(data []byte) error {
	type Alias Event
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(e),
	}
	return json.Unmarshal(data, &aux)
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
