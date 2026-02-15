package store

import (
	"encoding/json"
	"testing"
)

func TestEnsureRulesInitialized(t *testing.T) {
	if rs := ensureRulesInitialized(nil); rs == nil {
		t.Fatalf("expected non-nil slice")
	}
}

func TestUnmarshalTargetingRules(t *testing.T) {
	tests := []struct {
		name    string
		raw     json.RawMessage
		wantLen int
		wantErr bool
	}{
		{name: "nil", raw: nil, wantLen: 0},
		{name: "empty bytes", raw: json.RawMessage{}, wantLen: 0},
		{name: "null", raw: json.RawMessage("null"), wantLen: 0},
		{name: "empty array", raw: json.RawMessage("[]"), wantLen: 0},
		{
			name:    "single rule",
			raw:     json.RawMessage(`[{"id":"r1","conditions":[],"distribution":{"on":100}}]`),
			wantLen: 1,
		},
		{name: "invalid", raw: json.RawMessage("{"), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := unmarshalTargetingRules(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got == nil {
				t.Fatalf("expected non-nil slice")
			}
			if len(got) != tt.wantLen {
				t.Fatalf("expected len %d, got %d", tt.wantLen, len(got))
			}
		})
	}
}
