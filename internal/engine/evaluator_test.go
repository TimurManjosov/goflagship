package engine

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/TimurManjosov/goflagship/internal/rules"
	"github.com/TimurManjosov/goflagship/internal/store"
)

func TestOperatorHandlers(t *testing.T) {
	tests := []struct {
		name      string
		op        rules.Operator
		userValue any
		ruleValue any
		want      bool
	}{
		{name: "equals string true", op: rules.OpEq, userValue: "premium", ruleValue: "premium", want: true},
		{name: "equals string false", op: rules.Operator("equals"), userValue: "premium", ruleValue: "free", want: false},
		{name: "contains true", op: rules.OpContains, userValue: "premium_plan", ruleValue: "premium", want: true},
		{name: "starts_with true", op: rules.Operator("starts_with"), userValue: "premium_plan", ruleValue: "premium", want: true},
		{name: "ends_with true", op: rules.Operator("ends_with"), userValue: "premium_plan", ruleValue: "plan", want: true},
		{name: "regex true", op: rules.Operator("regex"), userValue: "user@example.com", ruleValue: `^[^@]+@example\.com$`, want: true},
		{name: "regex invalid pattern", op: rules.Operator("regex"), userValue: "abc", ruleValue: "(", want: false},
		{name: "gt int float64", op: rules.OpGt, userValue: 10, ruleValue: 9.5, want: true},
		{name: "lte float int", op: rules.OpLte, userValue: 10.0, ruleValue: 10, want: true},
		{name: "gte json number", op: rules.OpGte, userValue: json.Number("12"), ruleValue: 10, want: true},
		{name: "in_list []string", op: rules.OpIn, userValue: "US", ruleValue: []string{"US", "CA"}, want: true},
		{name: "not_in_list []any", op: rules.Operator("not_in_list"), userValue: "UK", ruleValue: []any{"US", "CA"}, want: true},
		{name: "semver gt", op: rules.OpSemVerGt, userValue: "1.2.0", ruleValue: "1.1.9", want: true},
		{name: "semver lt prerelease", op: rules.OpSemVerLt, userValue: "1.0.0-beta.1", ruleValue: "1.0.0", want: true},
		{name: "invalid type false", op: rules.OpContains, userValue: 123, ruleValue: "1", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, ok := getOperatorHandler(tt.op)
			if !ok {
				t.Fatalf("handler not found for %q", tt.op)
			}
			if got := handler.Check(tt.userValue, tt.ruleValue); got != tt.want {
				t.Fatalf("Check() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluate_BehaviorAndDeterminism(t *testing.T) {
	flag := &store.Flag{
		Key:     "new_checkout",
		Enabled: true,
		Config:  map[string]any{"enabled": false},
		Variants: []store.Variant{
			{Name: "control", Weight: 50, Config: map[string]any{"theme": "old"}},
			{Name: "treatment", Weight: 50, Config: map[string]any{"theme": "new"}},
		},
		TargetingRules: []rules.Rule{
			{
				ID: "rule-1",
				Conditions: []rules.Condition{
					{Property: "country", Operator: rules.OpEq, Value: "US"},
					{Property: "plan", Operator: rules.OpEq, Value: "premium"},
				},
				Distribution: map[string]int{"treatment": 100},
			},
			{
				ID: "rule-2",
				Conditions: []rules.Condition{
					{Property: "country", Operator: rules.OpEq, Value: "US"},
				},
				Distribution: map[string]int{"control": 100},
			},
		},
	}

	ctx := &UserContext{ID: "user-123", Country: "US", Plan: "premium"}

	got1 := Evaluate(flag, ctx)
	got2 := Evaluate(flag, ctx)

	if !reflect.DeepEqual(got1, got2) {
		t.Fatalf("Evaluate should be deterministic, got %#v and %#v", got1, got2)
	}
	if got1.Reason != string(ReasonTargetingMatch) {
		t.Fatalf("Reason = %s, want %s", got1.Reason, ReasonTargetingMatch)
	}
	if got1.MatchedRule != "rule-1" {
		t.Fatalf("MatchedRule = %s, want rule-1", got1.MatchedRule)
	}
	if got1.Variant != "treatment" {
		t.Fatalf("Variant = %s, want treatment", got1.Variant)
	}
}

func TestEvaluate_DisabledAndDefaultRollout(t *testing.T) {
	disabled := &store.Flag{Key: "f1", Enabled: false, Config: map[string]any{"enabled": false}}
	got := Evaluate(disabled, &UserContext{ID: "u1"})
	if got.Reason != string(ReasonDisabled) {
		t.Fatalf("Reason = %s, want %s", got.Reason, ReasonDisabled)
	}

	defaultFlag := &store.Flag{
		Key:     "f2",
		Enabled: true,
		Config:  map[string]any{"x": 1},
		Variants: []store.Variant{
			{Name: "control", Weight: 100, Config: map[string]any{"x": 2}},
		},
		TargetingRules: []rules.Rule{{
			ID: "rule-no-match",
			Conditions: []rules.Condition{
				{Property: "country", Operator: rules.OpEq, Value: "US"},
			},
			Distribution: map[string]int{"control": 100},
		}},
	}
	gotDefault := Evaluate(defaultFlag, &UserContext{ID: "u1", Country: "UK"})
	if gotDefault.Reason != string(ReasonDefaultRollout) {
		t.Fatalf("Reason = %s, want %s", gotDefault.Reason, ReasonDefaultRollout)
	}
}

func TestEvaluate_DistributionDeterminismAndMapOrder(t *testing.T) {
	flag := &store.Flag{
		Key:     "flag-map-order",
		Enabled: true,
		TargetingRules: []rules.Rule{{
			ID:           "r1",
			Conditions:   []rules.Condition{{Property: "country", Operator: rules.OpEq, Value: "US"}},
			Distribution: map[string]int{"b": 3000, "a": 7000},
		}},
	}
	ctx := &UserContext{ID: "fixed-user", Country: "US"}

	first := Evaluate(flag, ctx).Variant
	for i := 0; i < 50; i++ {
		if got := Evaluate(flag, ctx).Variant; got != first {
			t.Fatalf("variant changed across evaluations: first=%s got=%s", first, got)
		}
	}

	v1 := Evaluate(flag, &UserContext{ID: "user-1", Country: "US"}).Variant
	v2 := Evaluate(flag, &UserContext{ID: "user-2", Country: "US"}).Variant
	if v1 == "" || v2 == "" {
		t.Fatalf("expected non-empty variants for deterministic spread")
	}
	if hashBucket(flag.Key, &UserContext{ID: "user-1"}, nil, 10000) == hashBucket(flag.Key, &UserContext{ID: "user-2"}, nil, 10000) {
		t.Fatalf("expected different users to produce different buckets")
	}
}

func TestEvaluate_ContextLookupAndSafety(t *testing.T) {
	flag := &store.Flag{
		Key:     "ctx",
		Enabled: true,
		TargetingRules: []rules.Rule{{
			ID: "email-rule",
			Conditions: []rules.Condition{
				{Property: "email", Operator: rules.Operator("ends_with"), Value: "@example.com"},
				{Property: "team", Operator: rules.OpEq, Value: "growth"},
			},
			Distribution: map[string]int{"control": 100},
		}},
	}

	ctx := &UserContext{ID: "u1", Email: "a@example.com", Properties: map[string]any{"team": "growth"}}
	got := Evaluate(flag, ctx)
	if got.Reason != string(ReasonTargetingMatch) {
		t.Fatalf("Reason = %s, want %s", got.Reason, ReasonTargetingMatch)
	}

	nilRules := &store.Flag{Key: "empty", Enabled: true}
	_ = Evaluate(nilRules, ctx)

	unknownOperator := &store.Flag{
		Key:     "unknown-op",
		Enabled: true,
		TargetingRules: []rules.Rule{{
			ID:           "bad-op",
			Conditions:   []rules.Condition{{Property: "email", Operator: rules.Operator("unknown"), Value: "x"}},
			Distribution: map[string]int{"control": 100},
		}},
	}
	res := Evaluate(unknownOperator, ctx)
	if res.Reason != string(ReasonDefaultRollout) {
		t.Fatalf("unknown operator should fail condition and fallback to default rollout")
	}
}
