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

// ValidateFlag validates all flag fields and returns a validation result
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

// ValidateKey validates a flag key
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
		result.AddError("config", "Config must be valid JSON: "+err.Error())
		return result, nil
	}

	return result, config
}

// ValidateVariants validates a list of variants
func ValidateVariants(variants []VariantValidationParams) *ValidationResult {
	result := NewValidationResult()

	if len(variants) == 0 {
		return result
	}

	totalWeight := 0
	seenNames := make(map[string]bool)

	for i, v := range variants {
		// Validate name
		if strings.TrimSpace(v.Name) == "" {
			result.AddError("variants", "Variant name cannot be empty")
			continue
		}

		if utf8.RuneCountInString(v.Name) > MaxVariantNameLength {
			result.AddError("variants", "Variant name must not exceed 64 characters")
			continue
		}

		if seenNames[v.Name] {
			result.AddError("variants", "Duplicate variant name: "+v.Name)
			continue
		}
		seenNames[v.Name] = true

		// Validate weight
		if v.Weight < 0 || v.Weight > 100 {
			result.AddError("variants", "Variant weight must be between 0 and 100")
		}

		totalWeight += v.Weight

		// Prevent any other validation errors for same field
		if !result.Valid && i > 0 {
			break
		}
	}

	if result.Valid && totalWeight != 100 {
		result.AddError("variants", "Variant weights must sum to 100")
	}

	return result
}
