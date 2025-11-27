// Package targeting provides expression evaluation for feature flag targeting rules.
package targeting

import (
	"encoding/json"
	"errors"
	"strings"

	jsonlogic "github.com/diegoholiveira/jsonlogic/v3"
)

// UserContext represents the user attributes for targeting evaluation.
// Standard attributes include id, email, plan, country, etc.
// Custom attributes can be added via the attributes map.
type UserContext map[string]any

// ErrInvalidExpression is returned when an expression cannot be parsed as valid JSON Logic.
var ErrInvalidExpression = errors.New("invalid JSON Logic expression")

// ErrEvaluationFailed is returned when expression evaluation fails.
var ErrEvaluationFailed = errors.New("expression evaluation failed")

// Evaluate evaluates a JSON Logic expression against the user context.
// Returns true if the user matches the expression, false otherwise.
// If the expression is empty, returns true (no targeting).
func Evaluate(expression string, context UserContext) (bool, error) {
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return true, nil
	}

	// Parse the expression
	var logic any
	if err := json.Unmarshal([]byte(expression), &logic); err != nil {
		return false, ErrInvalidExpression
	}

	// Convert context directly to JSON (it's already map[string]any)
	dataJSON, err := json.Marshal(context)
	if err != nil {
		return false, ErrEvaluationFailed
	}

	// Apply JSON Logic
	result, err := jsonlogic.ApplyRaw([]byte(expression), dataJSON)
	if err != nil {
		return false, ErrEvaluationFailed
	}

	// Parse the result
	var boolResult bool
	if err := json.Unmarshal(result, &boolResult); err != nil {
		// Try to interpret non-boolean results
		// JSON Logic can return various types - treat truthy values as true
		var anyResult any
		if err := json.Unmarshal(result, &anyResult); err != nil {
			return false, ErrEvaluationFailed
		}
		return isTruthy(anyResult), nil
	}

	return boolResult, nil
}

// Validate checks if an expression is valid JSON Logic.
// Returns nil if valid, an error otherwise.
func Validate(expression string) error {
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return nil // Empty expression is valid (means no targeting)
	}

	// Check if it's valid JSON
	var logic any
	if err := json.Unmarshal([]byte(expression), &logic); err != nil {
		return ErrInvalidExpression
	}

	// Try to apply with empty data to catch invalid operators
	_, err := jsonlogic.ApplyRaw([]byte(expression), []byte("{}"))
	if err != nil {
		return ErrInvalidExpression
	}

	return nil
}

// isTruthy determines if a value is truthy according to JSON Logic semantics.
func isTruthy(v any) bool {
	if v == nil {
		return false
	}
	switch val := v.(type) {
	case bool:
		return val
	case float64:
		return val != 0
	case string:
		return val != ""
	case []any:
		return len(val) > 0
	case map[string]any:
		return len(val) > 0
	default:
		return true
	}
}
