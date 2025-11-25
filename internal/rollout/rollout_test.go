package rollout

import (
	"testing"
)

func TestIsRolledOut_Rollout0(t *testing.T) {
	// rollout=0 should always return false
	result, err := IsRolledOut("user-123", "feature_x", 0, "salt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("Expected false for rollout=0")
	}
}

func TestIsRolledOut_Rollout100(t *testing.T) {
	// rollout=100 should always return true
	result, err := IsRolledOut("user-123", "feature_x", 100, "salt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("Expected true for rollout=100")
	}
}

func TestIsRolledOut_EmptyUserID(t *testing.T) {
	// Empty userID should return false (no user context)
	result, err := IsRolledOut("", "feature_x", 50, "salt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("Expected false for empty userID")
	}
}

func TestIsRolledOut_InvalidRollout(t *testing.T) {
	// Invalid rollout values should return error
	_, err := IsRolledOut("user-123", "feature_x", -1, "salt")
	if err != ErrInvalidRollout {
		t.Errorf("Expected ErrInvalidRollout, got %v", err)
	}

	_, err = IsRolledOut("user-123", "feature_x", 101, "salt")
	if err != ErrInvalidRollout {
		t.Errorf("Expected ErrInvalidRollout, got %v", err)
	}
}

func TestIsRolledOut_Deterministic(t *testing.T) {
	// Same inputs should always return the same result
	userID := "user-123"
	flagKey := "feature_x"
	salt := "salt"
	rollout := int32(50)

	result1, _ := IsRolledOut(userID, flagKey, rollout, salt)
	result2, _ := IsRolledOut(userID, flagKey, rollout, salt)

	if result1 != result2 {
		t.Errorf("IsRolledOut is not deterministic: got %v and %v", result1, result2)
	}
}

func TestIsRolledOut_Distribution(t *testing.T) {
	// Test that ~50% of users are rolled out when rollout=50
	salt := "test-salt"
	flagKey := "feature_x"
	rollout := int32(50)
	rolledOutCount := 0
	total := 10000

	for i := 0; i < total; i++ {
		userID := "user-" + itoa(i)
		result, err := IsRolledOut(userID, flagKey, rollout, salt)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result {
			rolledOutCount++
		}
	}

	// Expect ~50% (5000), allow 5% variance (4500-5500)
	percentage := float64(rolledOutCount) / float64(total) * 100
	if percentage < 45 || percentage > 55 {
		t.Errorf("Expected ~50%% rollout, got %.2f%% (%d/%d)", percentage, rolledOutCount, total)
	}
}

func TestIsRolledOut_Distribution25(t *testing.T) {
	// Test that ~25% of users are rolled out when rollout=25
	salt := "test-salt"
	flagKey := "feature_25"
	rollout := int32(25)
	rolledOutCount := 0
	total := 10000

	for i := 0; i < total; i++ {
		userID := "user-" + itoa(i)
		result, _ := IsRolledOut(userID, flagKey, rollout, salt)
		if result {
			rolledOutCount++
		}
	}

	// Expect ~25% (2500), allow 5% variance (2000-3000)
	percentage := float64(rolledOutCount) / float64(total) * 100
	if percentage < 20 || percentage > 30 {
		t.Errorf("Expected ~25%% rollout, got %.2f%% (%d/%d)", percentage, rolledOutCount, total)
	}
}

func TestValidateVariants_Empty(t *testing.T) {
	err := ValidateVariants(nil)
	if err != nil {
		t.Errorf("Expected no error for empty variants, got %v", err)
	}

	err = ValidateVariants([]Variant{})
	if err != nil {
		t.Errorf("Expected no error for empty variants slice, got %v", err)
	}
}

func TestValidateVariants_ValidWeights(t *testing.T) {
	variants := []Variant{
		{Name: "control", Weight: 50},
		{Name: "experiment", Weight: 50},
	}
	err := ValidateVariants(variants)
	if err != nil {
		t.Errorf("Expected no error for valid variants, got %v", err)
	}
}

func TestValidateVariants_InvalidWeights(t *testing.T) {
	variants := []Variant{
		{Name: "control", Weight: 50},
		{Name: "experiment", Weight: 30},
	}
	err := ValidateVariants(variants)
	if err != ErrInvalidVariantWeights {
		t.Errorf("Expected ErrInvalidVariantWeights, got %v", err)
	}
}

func TestValidateVariants_DuplicateName(t *testing.T) {
	variants := []Variant{
		{Name: "control", Weight: 50},
		{Name: "control", Weight: 50},
	}
	err := ValidateVariants(variants)
	if err == nil {
		t.Error("Expected error for duplicate variant name")
	}
}

func TestValidateVariants_EmptyName(t *testing.T) {
	variants := []Variant{
		{Name: "", Weight: 50},
		{Name: "experiment", Weight: 50},
	}
	err := ValidateVariants(variants)
	if err == nil {
		t.Error("Expected error for empty variant name")
	}
}

func TestValidateVariants_NegativeWeight(t *testing.T) {
	variants := []Variant{
		{Name: "control", Weight: -10},
		{Name: "experiment", Weight: 110},
	}
	err := ValidateVariants(variants)
	if err == nil {
		t.Error("Expected error for negative weight")
	}
}

func TestGetVariant_EmptyVariants(t *testing.T) {
	variant, err := GetVariant("user-123", "feature_x", nil, "salt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if variant != "" {
		t.Errorf("Expected empty variant for empty variants, got %s", variant)
	}
}

func TestGetVariant_EmptyUserID(t *testing.T) {
	variants := []Variant{
		{Name: "control", Weight: 50},
		{Name: "experiment", Weight: 50},
	}
	variant, err := GetVariant("", "feature_x", variants, "salt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if variant != "" {
		t.Errorf("Expected empty variant for empty userID, got %s", variant)
	}
}

func TestGetVariant_Deterministic(t *testing.T) {
	variants := []Variant{
		{Name: "control", Weight: 50},
		{Name: "experiment", Weight: 50},
	}
	userID := "user-123"
	flagKey := "feature_x"
	salt := "salt"

	variant1, _ := GetVariant(userID, flagKey, variants, salt)
	variant2, _ := GetVariant(userID, flagKey, variants, salt)

	if variant1 != variant2 {
		t.Errorf("GetVariant is not deterministic: got %s and %s", variant1, variant2)
	}
}

func TestGetVariant_Distribution(t *testing.T) {
	variants := []Variant{
		{Name: "control", Weight: 50},
		{Name: "treatment", Weight: 30},
		{Name: "premium", Weight: 20},
	}
	salt := "test-salt"
	flagKey := "feature_x"
	counts := map[string]int{"control": 0, "treatment": 0, "premium": 0}
	total := 10000

	for i := 0; i < total; i++ {
		userID := "user-" + itoa(i)
		variant, err := GetVariant(userID, flagKey, variants, salt)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		counts[variant]++
	}

	// Check distribution with 5% tolerance
	checkVariantDistribution(t, counts, "control", 50, total)
	checkVariantDistribution(t, counts, "treatment", 30, total)
	checkVariantDistribution(t, counts, "premium", 20, total)
}

func checkVariantDistribution(t *testing.T, counts map[string]int, name string, expectedPct int, total int) {
	count := counts[name]
	actualPct := float64(count) / float64(total) * 100
	minPct := float64(expectedPct) - 5
	maxPct := float64(expectedPct) + 5

	if actualPct < minPct || actualPct > maxPct {
		t.Errorf("Variant %s: expected ~%d%%, got %.2f%% (%d/%d)", name, expectedPct, actualPct, count, total)
	}
}

func TestGetVariantConfig_ReturnsConfig(t *testing.T) {
	variants := []Variant{
		{Name: "control", Weight: 50, Config: map[string]any{"layout": "standard"}},
		{Name: "experiment", Weight: 50, Config: map[string]any{"layout": "new"}},
	}

	// Find a user that gets "control"
	var controlUser string
	for i := 0; i < 1000; i++ {
		userID := "user-" + itoa(i)
		variant, _ := GetVariant(userID, "feature_x", variants, "salt")
		if variant == "control" {
			controlUser = userID
			break
		}
	}

	if controlUser == "" {
		t.Skip("Could not find a user in control group")
	}

	config, err := GetVariantConfig(controlUser, "feature_x", variants, "salt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config == nil {
		t.Fatal("Expected config, got nil")
	}
	if config["layout"] != "standard" {
		t.Errorf("Expected layout=standard, got %v", config["layout"])
	}
}

func TestGetVariantConfig_EmptyUserID(t *testing.T) {
	variants := []Variant{
		{Name: "control", Weight: 100, Config: map[string]any{"value": 1}},
	}
	config, err := GetVariantConfig("", "feature_x", variants, "salt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config != nil {
		t.Errorf("Expected nil config for empty userID, got %v", config)
	}
}
