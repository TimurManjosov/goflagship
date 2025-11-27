package targeting

import (
	"testing"
)

func TestEvaluate_EmptyExpression(t *testing.T) {
	// Empty expression should always return true (no targeting)
	result, err := Evaluate("", UserContext{"id": "user-123"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("Expected true for empty expression")
	}

	// Whitespace-only expression
	result, err = Evaluate("   ", UserContext{"id": "user-123"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("Expected true for whitespace expression")
	}
}

func TestEvaluate_SimpleEquality(t *testing.T) {
	// Simple equality: plan == "premium"
	expression := `{"==": [{"var": "plan"}, "premium"]}`

	// Should match
	result, err := Evaluate(expression, UserContext{"plan": "premium"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("Expected true for premium user")
	}

	// Should not match
	result, err = Evaluate(expression, UserContext{"plan": "free"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("Expected false for free user")
	}
}

func TestEvaluate_InArray(t *testing.T) {
	// Check if country is in list
	expression := `{"in": [{"var": "country"}, ["US", "CA", "UK"]]}`

	// Should match
	result, err := Evaluate(expression, UserContext{"country": "US"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("Expected true for US user")
	}

	// Should not match
	result, err = Evaluate(expression, UserContext{"country": "FR"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("Expected false for FR user")
	}
}

func TestEvaluate_AndCondition(t *testing.T) {
	// Complex: premium AND in US
	expression := `{"and": [{"==": [{"var": "plan"}, "premium"]}, {"==": [{"var": "country"}, "US"]}]}`

	tests := []struct {
		name     string
		context  UserContext
		expected bool
	}{
		{"premium US user", UserContext{"plan": "premium", "country": "US"}, true},
		{"premium UK user", UserContext{"plan": "premium", "country": "UK"}, false},
		{"free US user", UserContext{"plan": "free", "country": "US"}, false},
		{"free UK user", UserContext{"plan": "free", "country": "UK"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Evaluate(expression, tt.context)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestEvaluate_OrCondition(t *testing.T) {
	// Premium OR beta tester
	expression := `{"or": [{"==": [{"var": "plan"}, "premium"]}, {"==": [{"var": "betaTester"}, true]}]}`

	tests := []struct {
		name     string
		context  UserContext
		expected bool
	}{
		{"premium user", UserContext{"plan": "premium", "betaTester": false}, true},
		{"beta tester", UserContext{"plan": "free", "betaTester": true}, true},
		{"premium beta tester", UserContext{"plan": "premium", "betaTester": true}, true},
		{"free non-tester", UserContext{"plan": "free", "betaTester": false}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Evaluate(expression, tt.context)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestEvaluate_NotCondition(t *testing.T) {
	// Not blocked
	expression := `{"!": {"==": [{"var": "blocked"}, true]}}`

	tests := []struct {
		name     string
		context  UserContext
		expected bool
	}{
		{"not blocked", UserContext{"blocked": false}, true},
		{"blocked", UserContext{"blocked": true}, false},
		{"no blocked field", UserContext{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Evaluate(expression, tt.context)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestEvaluate_NumericComparison(t *testing.T) {
	// App version >= 2.0
	expression := `{">=": [{"var": "appVersionNum"}, 2.0]}`

	tests := []struct {
		name     string
		context  UserContext
		expected bool
	}{
		{"version 2.5", UserContext{"appVersionNum": 2.5}, true},
		{"version 2.0", UserContext{"appVersionNum": 2.0}, true},
		{"version 1.9", UserContext{"appVersionNum": 1.9}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Evaluate(expression, tt.context)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestEvaluate_MissingVariable(t *testing.T) {
	// When variable doesn't exist, it's treated as null/undefined
	expression := `{"==": [{"var": "plan"}, "premium"]}`

	result, err := Evaluate(expression, UserContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("Expected false when variable is missing")
	}
}

func TestEvaluate_ComplexRealWorld(t *testing.T) {
	// Real-world scenario: Show new feature to premium users in US/CA OR beta testers
	expression := `{
		"or": [
			{
				"and": [
					{"==": [{"var": "plan"}, "premium"]},
					{"in": [{"var": "country"}, ["US", "CA"]]}
				]
			},
			{"==": [{"var": "betaTester"}, true]}
		]
	}`

	tests := []struct {
		name     string
		context  UserContext
		expected bool
	}{
		{"premium US", UserContext{"plan": "premium", "country": "US", "betaTester": false}, true},
		{"premium CA", UserContext{"plan": "premium", "country": "CA", "betaTester": false}, true},
		{"premium UK", UserContext{"plan": "premium", "country": "UK", "betaTester": false}, false},
		{"free US beta", UserContext{"plan": "free", "country": "US", "betaTester": true}, true},
		{"free UK beta", UserContext{"plan": "free", "country": "UK", "betaTester": true}, true},
		{"free UK", UserContext{"plan": "free", "country": "UK", "betaTester": false}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Evaluate(expression, tt.context)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestEvaluate_InvalidJSON(t *testing.T) {
	_, err := Evaluate("not valid json", UserContext{})
	if err != ErrInvalidExpression {
		t.Errorf("Expected ErrInvalidExpression, got %v", err)
	}
}

func TestValidate_ValidExpressions(t *testing.T) {
	validExpressions := []string{
		"",                                                 // Empty is valid
		`{"==": [{"var": "plan"}, "premium"]}`,             // Simple equality
		`{"and": [true, true]}`,                            // AND
		`{"or": [true, false]}`,                            // OR
		`{"!": false}`,                                     // NOT
		`{"in": [{"var": "x"}, ["a", "b", "c"]]}`,          // IN
		`{">=": [{"var": "version"}, 2]}`,                  // Comparison
		`{"if": [{"var": "x"}, "yes", "no"]}`,              // Conditional
	}

	for _, expr := range validExpressions {
		t.Run(expr, func(t *testing.T) {
			err := Validate(expr)
			if err != nil {
				t.Errorf("Expected valid expression, got error: %v", err)
			}
		})
	}
}

func TestValidate_InvalidExpressions(t *testing.T) {
	invalidExpressions := []string{
		"not json",         // Invalid JSON
		`{incomplete json`, // Incomplete JSON
	}

	for _, expr := range invalidExpressions {
		t.Run(expr, func(t *testing.T) {
			err := Validate(expr)
			if err == nil {
				t.Error("Expected error for invalid expression")
			}
		})
	}
}
