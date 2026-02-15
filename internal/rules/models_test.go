package rules

import (
	"encoding/json"
	"errors"
	"testing"
)

// ---------------------------------------------------------------------------
// JSON round-trip
// ---------------------------------------------------------------------------

func TestRuleJSONRoundtrip(t *testing.T) {
	original := Rule{
		ID: "rule-1",
		Conditions: []Condition{
			{Property: "email", Operator: OpContains, Value: "@firma.de"},
		},
		Distribution: map[string]int{"variant_a": 100},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded Rule
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.ID != original.ID {
		t.Errorf("ID: got %q, want %q", decoded.ID, original.ID)
	}
	if len(decoded.Conditions) != 1 {
		t.Fatalf("conditions length: got %d, want 1", len(decoded.Conditions))
	}
	c := decoded.Conditions[0]
	if c.Property != "email" {
		t.Errorf("property: got %q, want %q", c.Property, "email")
	}
	if c.Operator != OpContains {
		t.Errorf("operator: got %q, want %q", c.Operator, OpContains)
	}
	// After JSON round-trip, value is a string.
	if v, ok := c.Value.(string); !ok || v != "@firma.de" {
		t.Errorf("value: got %v (%T), want %q", c.Value, c.Value, "@firma.de")
	}
	if decoded.Distribution["variant_a"] != 100 {
		t.Errorf("distribution: got %v, want map[variant_a:100]", decoded.Distribution)
	}
}

// ---------------------------------------------------------------------------
// Validation — success cases
// ---------------------------------------------------------------------------

func TestValidateRule_Success(t *testing.T) {
	tests := []struct {
		name string
		rule Rule
	}{
		{
			name: "contains string, sum 100",
			rule: Rule{
				ID:           "r1",
				Conditions:   []Condition{{Property: "email", Operator: OpContains, Value: "@firma.de"}},
				Distribution: map[string]int{"variant_a": 100},
			},
		},
		{
			name: "in with []any, sum 100",
			rule: Rule{
				ID:           "r2",
				Conditions:   []Condition{{Property: "country", Operator: OpIn, Value: []any{"DE", "AT"}}},
				Distribution: map[string]int{"variant_a": 50, "variant_b": 50},
			},
		},
		{
			name: "gt numeric int, sum 10000",
			rule: Rule{
				ID:           "r3",
				Conditions:   []Condition{{Property: "age", Operator: OpGt, Value: 42}},
				Distribution: map[string]int{"variant_a": 10000},
			},
		},
		{
			name: "gt numeric float64, sum 100",
			rule: Rule{
				ID:           "r4",
				Conditions:   []Condition{{Property: "score", Operator: OpGte, Value: 9.5}},
				Distribution: map[string]int{"on": 100},
			},
		},
		{
			name: "eq with bool, sum 100",
			rule: Rule{
				ID:           "r5",
				Conditions:   []Condition{{Property: "beta", Operator: OpEq, Value: true}},
				Distribution: map[string]int{"on": 100},
			},
		},
		{
			name: "semver_gt string, sum 10000",
			rule: Rule{
				ID:           "r6",
				Conditions:   []Condition{{Property: "version", Operator: OpSemVerGt, Value: "1.2.3"}},
				Distribution: map[string]int{"on": 5000, "off": 5000},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateRule(tt.rule); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Validation — failure cases (table-driven)
// ---------------------------------------------------------------------------

func TestValidateRule_Failures(t *testing.T) {
	base := func(mods ...func(*Rule)) Rule {
		r := Rule{
			ID:           "r1",
			Conditions:   []Condition{{Property: "x", Operator: OpEq, Value: "hello"}},
			Distribution: map[string]int{"on": 100},
		}
		for _, m := range mods {
			m(&r)
		}
		return r
	}

	tests := []struct {
		name       string
		rule       Rule
		wantSentinel error
	}{
		{
			name:       "empty rule id",
			rule:       base(func(r *Rule) { r.ID = "" }),
			wantSentinel: ErrInvalidCondition,
		},
		{
			name:       "no conditions",
			rule:       base(func(r *Rule) { r.Conditions = nil }),
			wantSentinel: ErrInvalidCondition,
		},
		{
			name:       "empty property",
			rule:       base(func(r *Rule) { r.Conditions[0].Property = "" }),
			wantSentinel: ErrInvalidCondition,
		},
		{
			name:       "invalid operator",
			rule:       base(func(r *Rule) { r.Conditions[0].Operator = "nope" }),
			wantSentinel: ErrInvalidOperator,
		},
		{
			name:       "contains with non-string",
			rule:       base(func(r *Rule) { r.Conditions[0].Operator = OpContains; r.Conditions[0].Value = 42 }),
			wantSentinel: ErrInvalidValueType,
		},
		{
			name:       "in with non-slice",
			rule:       base(func(r *Rule) { r.Conditions[0].Operator = OpIn; r.Conditions[0].Value = "not-a-slice" }),
			wantSentinel: ErrInvalidValueType,
		},
		{
			name:       "gt with string",
			rule:       base(func(r *Rule) { r.Conditions[0].Operator = OpGt; r.Conditions[0].Value = "nope" }),
			wantSentinel: ErrInvalidValueType,
		},
		{
			name:       "distribution sum 90",
			rule:       base(func(r *Rule) { r.Distribution = map[string]int{"on": 90} }),
			wantSentinel: ErrDistributionSumIncorrect,
		},
		{
			name:       "distribution empty",
			rule:       base(func(r *Rule) { r.Distribution = map[string]int{} }),
			wantSentinel: ErrInvalidDistribution,
		},
		{
			name:       "distribution nil",
			rule:       base(func(r *Rule) { r.Distribution = nil }),
			wantSentinel: ErrInvalidDistribution,
		},
		{
			name:       "zero weight",
			rule:       base(func(r *Rule) { r.Distribution = map[string]int{"on": 0, "off": 100} }),
			wantSentinel: ErrInvalidDistribution,
		},
		{
			name:       "negative weight",
			rule:       base(func(r *Rule) { r.Distribution = map[string]int{"on": -10, "off": 110} }),
			wantSentinel: ErrInvalidDistribution,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRule(tt.rule)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !errors.Is(err, tt.wantSentinel) {
				t.Errorf("error = %v; want sentinel %v", err, tt.wantSentinel)
			}
		})
	}
}
