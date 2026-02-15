package rules

import (
	"errors"
	"fmt"
)

// Sentinel errors returned by ValidateRule.
var (
	ErrInvalidOperator        = errors.New("invalid operator")
	ErrInvalidCondition       = errors.New("invalid condition")
	ErrInvalidValueType       = errors.New("invalid value type")
	ErrInvalidDistribution    = errors.New("invalid distribution")
	ErrDistributionSumIncorrect = errors.New("distribution sum incorrect")
)

// validOperators is the set of all recognised targeting operators.
var validOperators = map[Operator]struct{}{
	OpEq:       {},
	OpNeq:      {},
	OpContains: {},
	OpIn:       {},
	OpGt:       {},
	OpLt:       {},
	OpGte:      {},
	OpLte:      {},
	OpSemVerGt: {},
	OpSemVerLt: {},
}

// ValidateRule performs strict validation of a targeting Rule.
// It is a pure function: it never mutates r and has no side effects.
func ValidateRule(r Rule) error {
	if r.ID == "" {
		return fmt.Errorf("%w: rule id must not be empty", ErrInvalidCondition)
	}

	if len(r.Conditions) == 0 {
		return fmt.Errorf("%w: rule must have at least one condition", ErrInvalidCondition)
	}

	for i, c := range r.Conditions {
		if err := validateCondition(i, c); err != nil {
			return err
		}
	}

	return validateDistribution(r.Distribution)
}

func validateCondition(i int, c Condition) error {
	if c.Property == "" {
		return fmt.Errorf("%w: condition[%d] property must not be empty", ErrInvalidCondition, i)
	}

	if _, ok := validOperators[c.Operator]; !ok {
		return fmt.Errorf("%w: condition[%d] operator %q is not supported", ErrInvalidOperator, i, c.Operator)
	}

	return validateValueType(i, c.Operator, c.Value)
}

// validateValueType checks that the condition value has a type compatible with
// the operator. It uses explicit type assertions — no reflection.
func validateValueType(i int, op Operator, v interface{}) error {
	switch op {
	case OpContains, OpSemVerGt, OpSemVerLt:
		if _, ok := v.(string); !ok {
			return fmt.Errorf("%w: condition[%d] operator %q requires a string value", ErrInvalidValueType, i, op)
		}

	case OpIn:
		if !isSlice(v) {
			return fmt.Errorf("%w: condition[%d] operator %q requires a slice value", ErrInvalidValueType, i, op)
		}

	case OpGt, OpLt, OpGte, OpLte:
		if !isNumeric(v) {
			return fmt.Errorf("%w: condition[%d] operator %q requires a numeric value", ErrInvalidValueType, i, op)
		}

	case OpEq, OpNeq:
		if !isScalar(v) {
			return fmt.Errorf("%w: condition[%d] operator %q requires a scalar value (string, bool, or number)", ErrInvalidValueType, i, op)
		}
	}

	return nil
}

// isSlice returns true for slice types that may appear after JSON unmarshaling
// or be provided programmatically.
func isSlice(v interface{}) bool {
	switch v.(type) {
	case []any, []string, []int, []float64:
		return true
	}
	return false
}

// isNumeric returns true for integer and floating-point types.
func isNumeric(v interface{}) bool {
	switch v.(type) {
	case int, int32, int64, float32, float64:
		return true
	}
	return false
}

// isScalar returns true for basic scalar types (string, bool, numeric).
func isScalar(v interface{}) bool {
	switch v.(type) {
	case string, bool, int, int32, int64, float32, float64:
		return true
	}
	return false
}

func validateDistribution(d map[string]int) error {
	if len(d) == 0 {
		return fmt.Errorf("%w: distribution must not be empty", ErrInvalidDistribution)
	}

	sum := 0
	for variant, weight := range d {
		if weight <= 0 {
			return fmt.Errorf("%w: variant %q has non-positive weight %d", ErrInvalidDistribution, variant, weight)
		}
		sum += weight
	}

	if sum != 100 && sum != 10000 {
		return fmt.Errorf("%w: got %d, want 100 (percent) or 10000 (basis points)", ErrDistributionSumIncorrect, sum)
	}

	return nil
}
