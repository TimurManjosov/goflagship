package evaluation

import (
	"testing"

	"github.com/TimurManjosov/goflagship/internal/snapshot"
)

func TestEvaluateFlag_DisabledFlag(t *testing.T) {
	flag := snapshot.FlagView{
		Key:     "disabled_flag",
		Enabled: false,
		Rollout: 100,
	}
	ctx := Context{UserID: "user-123"}
	salt := "test-salt"

	result := EvaluateFlag(flag, ctx, salt)

	if result.Enabled {
		t.Error("Expected disabled flag to evaluate to false")
	}
	if result.Key != "disabled_flag" {
		t.Errorf("Expected key 'disabled_flag', got %s", result.Key)
	}
}

func TestEvaluateFlag_EnabledSimpleFlag(t *testing.T) {
	flag := snapshot.FlagView{
		Key:     "enabled_flag",
		Enabled: true,
		Rollout: 100,
	}
	ctx := Context{UserID: "user-123"}
	salt := "test-salt"

	result := EvaluateFlag(flag, ctx, salt)

	if !result.Enabled {
		t.Error("Expected enabled flag with 100% rollout to evaluate to true")
	}
}

func TestEvaluateFlag_WithConfig(t *testing.T) {
	config := map[string]any{"color": "blue", "size": 10}
	flag := snapshot.FlagView{
		Key:     "config_flag",
		Enabled: true,
		Rollout: 100,
		Config:  config,
	}
	ctx := Context{UserID: "user-123"}
	salt := "test-salt"

	result := EvaluateFlag(flag, ctx, salt)

	if !result.Enabled {
		t.Error("Expected flag to be enabled")
	}
	if result.Config["color"] != "blue" {
		t.Errorf("Expected color 'blue', got %v", result.Config["color"])
	}
	if result.Config["size"] != 10 {
		t.Errorf("Expected size 10, got %v", result.Config["size"])
	}
}

func TestEvaluateFlag_WithExpression_Match(t *testing.T) {
	expr := `{"==": [{"var": "plan"}, "premium"]}`
	flag := snapshot.FlagView{
		Key:        "expr_flag",
		Enabled:    true,
		Rollout:    100,
		Expression: &expr,
	}
	ctx := Context{
		UserID:     "user-123",
		Attributes: map[string]any{"plan": "premium"},
	}
	salt := "test-salt"

	result := EvaluateFlag(flag, ctx, salt)

	if !result.Enabled {
		t.Error("Expected flag to be enabled for matching expression")
	}
}

func TestEvaluateFlag_WithExpression_NoMatch(t *testing.T) {
	expr := `{"==": [{"var": "plan"}, "premium"]}`
	flag := snapshot.FlagView{
		Key:        "expr_flag",
		Enabled:    true,
		Rollout:    100,
		Expression: &expr,
	}
	ctx := Context{
		UserID:     "user-123",
		Attributes: map[string]any{"plan": "free"},
	}
	salt := "test-salt"

	result := EvaluateFlag(flag, ctx, salt)

	if result.Enabled {
		t.Error("Expected flag to be disabled for non-matching expression")
	}
}

func TestEvaluateFlag_WithRollout_ZeroPercent(t *testing.T) {
	flag := snapshot.FlagView{
		Key:     "rollout_flag",
		Enabled: true,
		Rollout: 0,
	}
	ctx := Context{UserID: "user-123"}
	salt := "test-salt"

	result := EvaluateFlag(flag, ctx, salt)

	if result.Enabled {
		t.Error("Expected flag with 0% rollout to be disabled")
	}
}

func TestEvaluateFlag_WithRollout_Deterministic(t *testing.T) {
	flag := snapshot.FlagView{
		Key:     "rollout_flag",
		Enabled: true,
		Rollout: 50,
	}
	ctx := Context{UserID: "user-123"}
	salt := "test-salt"

	// Same inputs should produce same result
	result1 := EvaluateFlag(flag, ctx, salt)
	result2 := EvaluateFlag(flag, ctx, salt)

	if result1.Enabled != result2.Enabled {
		t.Error("Expected deterministic rollout evaluation")
	}
}

func TestEvaluateFlag_WithVariants(t *testing.T) {
	flag := snapshot.FlagView{
		Key:     "variant_flag",
		Enabled: true,
		Rollout: 100,
		Variants: []snapshot.Variant{
			{Name: "control", Weight: 50, Config: map[string]any{"color": "red"}},
			{Name: "treatment", Weight: 50, Config: map[string]any{"color": "blue"}},
		},
	}
	ctx := Context{UserID: "user-123"}
	salt := "test-salt"

	result := EvaluateFlag(flag, ctx, salt)

	if !result.Enabled {
		t.Error("Expected flag to be enabled")
	}
	if result.Variant != "control" && result.Variant != "treatment" {
		t.Errorf("Expected variant to be 'control' or 'treatment', got %s", result.Variant)
	}
	if result.Config == nil {
		t.Error("Expected variant config to be present")
	}
}

func TestEvaluateFlag_WithVariants_Deterministic(t *testing.T) {
	flag := snapshot.FlagView{
		Key:     "variant_flag",
		Enabled: true,
		Rollout: 100,
		Variants: []snapshot.Variant{
			{Name: "control", Weight: 50},
			{Name: "treatment", Weight: 50},
		},
	}
	ctx := Context{UserID: "user-123"}
	salt := "test-salt"

	result1 := EvaluateFlag(flag, ctx, salt)
	result2 := EvaluateFlag(flag, ctx, salt)

	if result1.Variant != result2.Variant {
		t.Error("Expected deterministic variant selection")
	}
}

func TestEvaluateFlag_EmptyUserID_NoRollout(t *testing.T) {
	flag := snapshot.FlagView{
		Key:     "rollout_flag",
		Enabled: true,
		Rollout: 50,
	}
	ctx := Context{UserID: ""} // Empty user ID
	salt := "test-salt"

	result := EvaluateFlag(flag, ctx, salt)

	if result.Enabled {
		t.Error("Expected flag with rollout and empty user ID to be disabled")
	}
}

func TestEvaluateAll_AllFlags(t *testing.T) {
	flags := map[string]snapshot.FlagView{
		"flag1": {Key: "flag1", Enabled: true, Rollout: 100},
		"flag2": {Key: "flag2", Enabled: false, Rollout: 100},
		"flag3": {Key: "flag3", Enabled: true, Rollout: 100},
	}
	ctx := Context{UserID: "user-123"}
	salt := "test-salt"

	results := EvaluateAll(flags, ctx, salt, nil)

	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}

	// Check that we have all flags
	foundFlags := make(map[string]bool)
	for _, r := range results {
		foundFlags[r.Key] = true
	}
	for key := range flags {
		if !foundFlags[key] {
			t.Errorf("Expected flag %s in results", key)
		}
	}
}

func TestEvaluateAll_FilteredByKeys(t *testing.T) {
	flags := map[string]snapshot.FlagView{
		"flag1": {Key: "flag1", Enabled: true, Rollout: 100},
		"flag2": {Key: "flag2", Enabled: true, Rollout: 100},
		"flag3": {Key: "flag3", Enabled: true, Rollout: 100},
	}
	ctx := Context{UserID: "user-123"}
	salt := "test-salt"
	keys := []string{"flag1", "flag3"}

	results := EvaluateAll(flags, ctx, salt, keys)

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	// Check that only requested flags are present
	foundFlags := make(map[string]bool)
	for _, r := range results {
		foundFlags[r.Key] = true
	}
	if !foundFlags["flag1"] || !foundFlags["flag3"] {
		t.Error("Expected flag1 and flag3 in results")
	}
	if foundFlags["flag2"] {
		t.Error("Did not expect flag2 in results")
	}
}

func TestEvaluateAll_NonExistentKey(t *testing.T) {
	flags := map[string]snapshot.FlagView{
		"flag1": {Key: "flag1", Enabled: true, Rollout: 100},
	}
	ctx := Context{UserID: "user-123"}
	salt := "test-salt"
	keys := []string{"flag1", "nonexistent"}

	results := EvaluateAll(flags, ctx, salt, keys)

	// Should only return the existing flag
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}
	if results[0].Key != "flag1" {
		t.Errorf("Expected flag1, got %s", results[0].Key)
	}
}

func TestEvaluateAll_EmptyFlags(t *testing.T) {
	flags := map[string]snapshot.FlagView{}
	ctx := Context{UserID: "user-123"}
	salt := "test-salt"

	results := EvaluateAll(flags, ctx, salt, nil)

	if len(results) != 0 {
		t.Errorf("Expected 0 results, got %d", len(results))
	}
}

func TestBuildTargetingContext(t *testing.T) {
	ctx := Context{
		UserID: "user-123",
		Attributes: map[string]any{
			"plan":    "premium",
			"country": "US",
			"age":     25,
		},
	}

	targetCtx := buildTargetingContext(ctx)

	if targetCtx["id"] != "user-123" {
		t.Errorf("Expected id 'user-123', got %v", targetCtx["id"])
	}
	if targetCtx["plan"] != "premium" {
		t.Errorf("Expected plan 'premium', got %v", targetCtx["plan"])
	}
	if targetCtx["country"] != "US" {
		t.Errorf("Expected country 'US', got %v", targetCtx["country"])
	}
	if targetCtx["age"] != 25 {
		t.Errorf("Expected age 25, got %v", targetCtx["age"])
	}
}

func TestEvaluateFlag_ComplexScenario(t *testing.T) {
	// Test the full evaluation flow: enabled → expression → rollout → variant
	expr := `{"and": [{"==": [{"var": "plan"}, "premium"]}, {">=": [{"var": "age"}, 18]}]}`
	flag := snapshot.FlagView{
		Key:        "complex_flag",
		Enabled:    true,
		Rollout:    100,
		Expression: &expr,
		Config:     map[string]any{"default": true},
		Variants: []snapshot.Variant{
			{Name: "control", Weight: 50, Config: map[string]any{"color": "red"}},
			{Name: "treatment", Weight: 50, Config: map[string]any{"color": "blue"}},
		},
	}
	ctx := Context{
		UserID: "user-123",
		Attributes: map[string]any{
			"plan": "premium",
			"age":  25,
		},
	}
	salt := "test-salt"

	result := EvaluateFlag(flag, ctx, salt)

	if !result.Enabled {
		t.Error("Expected flag to be enabled for premium user over 18")
	}
	if result.Variant == "" {
		t.Error("Expected a variant to be selected")
	}
	if result.Config == nil {
		t.Error("Expected variant config to be present")
	}
}
