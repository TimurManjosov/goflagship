// Package targeting provides expression evaluation for feature flag targeting rules.
// It uses JSON Logic (jsonlogic.com) for evaluating rules against user context.
package targeting

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"

	"github.com/diegoholiveira/jsonlogic/v3"
)

// UserContext represents user attributes for targeting evaluation.
// Common attributes include:
//   - id: user identifier (string)
//   - email: user email address (string)
//   - plan: subscription plan, e.g., "free", "premium", "enterprise" (string)
//   - country: ISO 3166-1 alpha-2 country code, e.g., "US", "CA" (string)
//   - age: user age (number)
//   - isBeta: whether user is a beta tester (bool)
//   - customAttributes: any additional attributes
type UserContext map[string]any

// ErrInvalidExpression is returned when an expression is not valid JSON Logic.
var ErrInvalidExpression = errors.New("invalid expression: not valid JSON Logic")

// ErrEmptyExpression is returned when an expression is empty or whitespace.
var ErrEmptyExpression = errors.New("invalid expression: empty or whitespace")

// Evaluate evaluates a JSON Logic expression against a user context.
// Returns true if the user matches the expression, false otherwise.
// Returns an error if the expression is invalid.
func Evaluate(expression string, ctx UserContext) (bool, error) {
	if strings.TrimSpace(expression) == "" {
		return false, ErrEmptyExpression
	}

	// Convert context to JSON
	dataBytes, err := json.Marshal(ctx)
	if err != nil {
		return false, err
	}

	// Create readers for the JSON Logic library
	ruleReader := strings.NewReader(expression)
	dataReader := bytes.NewReader(dataBytes)
	var resultBuf bytes.Buffer

	// Apply the rule - this will fail if expression is not valid JSON
	if err := jsonlogic.Apply(ruleReader, dataReader, &resultBuf); err != nil {
		return false, ErrInvalidExpression
	}

	// Parse result
	var result any
	if err := json.Unmarshal(resultBuf.Bytes(), &result); err != nil {
		return false, err
	}

	// Convert to bool following JavaScript-like truthiness
	return isTruthy(result), nil
}

// ValidateExpression checks if an expression is valid JSON Logic.
// Returns nil if valid, or an error describing why it's invalid.
func ValidateExpression(expression string) error {
	if strings.TrimSpace(expression) == "" {
		return ErrEmptyExpression
	}

	// Check if it's valid JSON first
	var rule any
	if err := json.Unmarshal([]byte(expression), &rule); err != nil {
		return ErrInvalidExpression
	}

	// Try to validate by applying against empty data
	ruleReader := strings.NewReader(expression)
	dataReader := strings.NewReader("{}")
	var resultBuf bytes.Buffer

	if err := jsonlogic.Apply(ruleReader, dataReader, &resultBuf); err != nil {
		return ErrInvalidExpression
	}

	return nil
}

// isTruthy follows JavaScript-like truthiness rules.
// Returns true for non-zero numbers, non-empty strings, non-empty arrays/objects, and true boolean.
func isTruthy(v any) bool {
	if v == nil {
		return false
	}
	switch val := v.(type) {
	case bool:
		return val
	case float64:
		return val != 0
	case int:
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
