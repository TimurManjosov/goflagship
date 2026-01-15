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
//
// Preconditions:
//   - expression should be valid JSON Logic (validation recommended via ValidateExpression)
//   - ctx may be nil or empty (will be treated as empty context)
//
// Postconditions:
//   - Returns (true, nil) if user matches expression
//   - Returns (false, nil) if user doesn't match expression
//   - Returns (false, error) if expression is invalid or evaluation fails
//   - Never returns (true, non-nil error)
//
// Expression Format:
//   Uses JSON Logic (jsonlogic.com) syntax. Examples:
//   - {"==": [{"var": "plan"}, "premium"]} — checks if user.plan == "premium"
//   - {"in": [{"var": "country"}, ["US", "CA"]]} — checks if user.country in ["US", "CA"]
//   - {"and": [...]} — combines multiple conditions
//
// Result Interpretation:
//   Uses JavaScript-like truthiness rules:
//   - true: non-zero numbers, non-empty strings, non-empty arrays/objects, boolean true
//   - false: null, 0, "", [], {}, boolean false
//
// Edge Cases:
//   - expression is empty or whitespace: Returns (false, ErrEmptyExpression)
//   - expression is invalid JSON: Returns (false, ErrInvalidExpression)
//   - expression is valid JSON but invalid JSON Logic: Returns (false, ErrInvalidExpression)
//   - ctx is nil: Treated as empty context {}
//   - ctx has no matching keys: Expression may still evaluate (depends on logic)
//   - expression references non-existent ctx keys: Treated as undefined (falsy)
//
// Error Cases:
//   - ErrEmptyExpression: expression is empty or whitespace only
//   - ErrInvalidExpression: expression is not valid JSON or JSON Logic
//   - Other errors: JSON marshaling failures (rare)
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
//
// Preconditions:
//   - expression may be any string
//
// Postconditions:
//   - Returns nil if expression is valid JSON Logic
//   - Returns error describing why expression is invalid
//   - Does not evaluate expression against real data (uses empty context)
//
// Validation Steps:
//   1. Check if expression is empty/whitespace → ErrEmptyExpression
//   2. Check if expression is valid JSON → ErrInvalidExpression
//   3. Check if expression is valid JSON Logic → ErrInvalidExpression
//
// Edge Cases:
//   - expression is empty: Returns ErrEmptyExpression
//   - expression is whitespace only: Returns ErrEmptyExpression
//   - expression is valid JSON but invalid JSON Logic: Returns ErrInvalidExpression
//   - expression is "{}" (empty object): Valid (always evaluates to falsy)
//   - expression references undefined variables: Valid (variables evaluated at runtime)
//
// Usage:
//   Use this before storing expressions in database to catch syntax errors early.
//   Does not validate that expression makes semantic sense for your domain.
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
