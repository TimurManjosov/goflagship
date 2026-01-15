// Package validation provides validation rules for flag data and request parameters.
package validation

import (
	"encoding/json"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	// MaxKeyLength is the maximum length for flag keys
	MaxKeyLength = 64
	// MaxEnvLength is the maximum length for environment names
	MaxEnvLength = 32
	// MaxDescriptionLength is the maximum length for flag descriptions
	MaxDescriptionLength = 500
	// MaxConfigSize is the maximum size of config JSON in bytes
	MaxConfigSize = 100 * 1024 // 100KB
	// MinRollout is the minimum rollout percentage
	MinRollout = 0
	// MaxRollout is the maximum rollout percentage
	MaxRollout = 100
	// MaxVariantNameLength is the maximum length for variant names
	MaxVariantNameLength = 64
)

// keyPattern matches alphanumeric characters, underscores, and hyphens
var keyPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// ValidationResult holds the result of validation
type ValidationResult struct {
	Valid  bool
	Errors map[string]string
}

// NewValidationResult creates a new validation result
func NewValidationResult() *ValidationResult {
	return &ValidationResult{
		Valid:  true,
		Errors: make(map[string]string),
	}
}

// AddError adds a field error and marks the result as invalid
func (v *ValidationResult) AddError(field, message string) {
	v.Valid = false
	v.Errors[field] = message
}

// Merge combines another validation result into this one
func (v *ValidationResult) Merge(other *ValidationResult) {
	if other == nil {
		return
	}
	for field, message := range other.Errors {
		v.AddError(field, message)
	}
}

// FlagValidationParams contains the parameters for validating a flag
type FlagValidationParams struct {
	Key         string
	Env         string
	Description string
	Rollout     int32
	Config      map[string]any
	ConfigJSON  string // Raw JSON string for size validation
	Variants    []VariantValidationParams
	Expression  *string
}

// VariantValidationParams contains the parameters for validating a variant
type VariantValidationParams struct {
	Name   string
	Weight int
}

// ValidateFlag validates all flag fields and returns a validation result.
//
// Preconditions:
//   - params contains the flag data to validate
//   - params fields may have any values (validation will check all constraints)
//
// Postconditions:
//   - Always returns non-nil *ValidationResult
//   - result.Valid is true if all validations pass
//   - result.Errors contains field-specific error messages if validation fails
//   - Does not modify params
//
// Validation Order:
//   1. Key validation (required, max length, pattern)
//   2. Env validation (required, max length)
//   3. Description validation (max length)
//   4. Rollout validation (range 0-100)
//   5. Config size validation (if ConfigJSON provided)
//   6. Variants validation (if Variants provided)
//
// Edge Cases:
//   - All required fields (e.g., Key, Env) empty: Multiple validation errors returned for those fields
//   - Some fields valid, some invalid: Only invalid fields have errors
//   - ConfigJSON empty: Config size validation skipped
//   - Variants empty: Variant validation skipped
//   - Multiple validation failures: All failures are collected and returned
//
// Usage Pattern:
//   result := ValidateFlag(params)
//   if !result.Valid {
//       // Handle validation errors in result.Errors map
//   }
func ValidateFlag(params FlagValidationParams) *ValidationResult {
	result := NewValidationResult()

	// Validate key
	keyResult := ValidateKey(params.Key)
	result.Merge(keyResult)

	// Validate env
	envResult := ValidateEnv(params.Env)
	result.Merge(envResult)

	// Validate description
	descResult := ValidateDescription(params.Description)
	result.Merge(descResult)

	// Validate rollout
	rolloutResult := ValidateRollout(params.Rollout)
	result.Merge(rolloutResult)

	// Validate config size if raw JSON is provided
	if params.ConfigJSON != "" {
		configResult := ValidateConfigSize(params.ConfigJSON)
		result.Merge(configResult)
	}

	// Validate variants if provided
	if len(params.Variants) > 0 {
		variantsResult := ValidateVariants(params.Variants)
		result.Merge(variantsResult)
	}

	return result
}

// ValidateKey validates a flag key.
//
// Preconditions:
//   - key may be any string (including empty)
//
// Postconditions:
//   - Returns *ValidationResult with Valid=true if key passes all checks
//   - Returns *ValidationResult with Valid=false and error in Errors["key"] if invalid
//
// Validation Rules:
//   1. Key cannot be empty (after trimming whitespace)
//   2. Key must not exceed MaxKeyLength (64) characters
//   3. Key must match pattern: alphanumeric, underscores, hyphens only
//
// Edge Cases:
//   - key is empty string: Error "Key is required"
//   - key is whitespace only: Error "Key is required" (trimmed to empty)
//   - key has spaces in middle: Error about invalid pattern
//   - key has special characters: Error about invalid pattern
//   - key exactly 64 chars: Valid
//   - key 65 chars: Error about length
func ValidateKey(key string) *ValidationResult {
	result := NewValidationResult()
	key = strings.TrimSpace(key)

	if key == "" {
		result.AddError("key", "Key is required")
		return result
	}

	if utf8.RuneCountInString(key) > MaxKeyLength {
		result.AddError("key", "Key must not exceed 64 characters")
		return result
	}

	if !keyPattern.MatchString(key) {
		result.AddError("key", "Key must contain only alphanumeric characters, underscores, and hyphens")
		return result
	}

	return result
}

// ValidateEnv validates an environment name
func ValidateEnv(env string) *ValidationResult {
	result := NewValidationResult()
	env = strings.TrimSpace(env)

	if env == "" {
		result.AddError("env", "Environment is required")
		return result
	}

	if utf8.RuneCountInString(env) > MaxEnvLength {
		result.AddError("env", "Environment must not exceed 32 characters")
		return result
	}

	return result
}

// ValidateDescription validates a flag description
func ValidateDescription(description string) *ValidationResult {
	result := NewValidationResult()

	if utf8.RuneCountInString(description) > MaxDescriptionLength {
		result.AddError("description", "Description must not exceed 500 characters")
	}

	return result
}

// ValidateRollout validates a rollout percentage
func ValidateRollout(rollout int32) *ValidationResult {
	result := NewValidationResult()

	if rollout < MinRollout || rollout > MaxRollout {
		result.AddError("rollout", "Rollout must be between 0 and 100")
	}

	return result
}

// ValidateConfigSize validates the config JSON size
func ValidateConfigSize(configJSON string) *ValidationResult {
	result := NewValidationResult()

	if len(configJSON) > MaxConfigSize {
		result.AddError("config", "Config must not exceed 100KB")
	}

	return result
}

// ValidateConfigJSON validates that config is valid JSON object
func ValidateConfigJSON(configStr string) (*ValidationResult, map[string]any) {
	result := NewValidationResult()

	if strings.TrimSpace(configStr) == "" {
		return result, nil
	}

	var config map[string]any
	if err := json.Unmarshal([]byte(configStr), &config); err != nil {
		result.AddError("config", "Config must be a valid JSON object")
		return result, nil
	}

	return result, config
}

// ValidateVariants validates a list of variants.
//
// Preconditions:
//   - variants may be nil or empty slice
//
// Postconditions:
//   - Returns *ValidationResult with Valid=true for empty or valid variants
//   - Returns *ValidationResult with Valid=false if any validation fails
//   - Only first error is reported (stops on first validation failure)
//
// Validation Rules:
//   1. Empty variants slice is valid (no A/B test)
//   2. Each variant name must be non-empty after trimming
//   3. Each variant name must not exceed MaxVariantNameLength (64)
//   4. All variant names must be unique
//   5. Each variant weight must be in range [0, 100]
//   6. Sum of all variant weights must equal exactly 100
//
// Edge Cases:
//   - variants is nil: Valid (no A/B test)
//   - variants is empty slice: Valid (no A/B test)
//   - Single variant with weight=100: Valid (control-only test)
//   - Variants with weights summing to 99: Invalid (must sum to 100)
//   - Variants with weights summing to 101: Invalid (must sum to 100)
//   - Duplicate variant names: Invalid (must be unique)
//   - Variant with empty name: Invalid (name required)
//   - Variant with name > 64 chars: Invalid (exceeds max length)
//
// Error Reporting:
//   Stops at first validation error and returns it in Errors["variants"].
//   Does not collect multiple variant errors in one pass.
func ValidateVariants(variants []VariantValidationParams) *ValidationResult {
	result := NewValidationResult()

	if len(variants) == 0 {
		return result
	}

	totalWeight := 0
	seenNames := make(map[string]bool)
	hasValidationError := false

	for _, v := range variants {
		// Validate name
		if strings.TrimSpace(v.Name) == "" {
			if !hasValidationError {
				result.AddError("variants", "Variant name cannot be empty")
				hasValidationError = true
			}
			continue
		}

		if utf8.RuneCountInString(v.Name) > MaxVariantNameLength {
			if !hasValidationError {
				result.AddError("variants", "Variant name must not exceed 64 characters")
				hasValidationError = true
			}
			continue
		}

		if seenNames[v.Name] {
			if !hasValidationError {
				result.AddError("variants", "Duplicate variant name: "+v.Name)
				hasValidationError = true
			}
			continue
		}
		seenNames[v.Name] = true

		// Validate weight
		if v.Weight < 0 || v.Weight > 100 {
			if !hasValidationError {
				result.AddError("variants", "Variant weight must be between 0 and 100")
				hasValidationError = true
			}
		}

		totalWeight += v.Weight
	}

	if !hasValidationError && totalWeight != 100 {
		result.AddError("variants", "Variant weights must sum to 100")
	}

	return result
}
