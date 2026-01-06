// Package rollout provides deterministic user bucketing for feature flag rollouts.
// It uses consistent hashing to assign users to buckets (0-99) based on their user ID,
// flag key, and a secret salt. This ensures:
//   - Same user always gets same result for a flag (deterministic)
//   - Even distribution across buckets (uses xxHash algorithm)
//   - Consistency between server and client evaluation (when using same salt)
//   - Safe progressive rollouts (increasing from 10% to 20% only adds users, never removes)
package rollout

import "errors"

// ErrInvalidRollout is returned when the rollout percentage is not in the valid range (0-100).
var ErrInvalidRollout = errors.New("rollout must be between 0 and 100")

// ErrInvalidVariantWeights is returned when variant weights don't sum to 100.
var ErrInvalidVariantWeights = errors.New("variant weights must sum to 100")

// Variant represents a variant in an A/B test or multi-variant experiment.
type Variant struct {
	Name   string         `json:"name"`
	Weight int            `json:"weight"`           // Percentage weight (0-100)
	Config map[string]any `json:"config,omitempty"` // Optional config for this variant
}

// IsRolledOut determines if a user is included in a feature flag rollout.
//
// Algorithm:
//   1. Hash(userID + flagKey + salt) → bucket (0-99)
//   2. If bucket < rollout percentage, user is included
//
// Special cases:
//   - rollout=0: Always returns false (flag disabled for all)
//   - rollout=100: Always returns true (flag enabled for all)
//   - userID="": Always returns false (no user context means no targeting)
//
// Example: rollout=25 means ~25% of users see the feature.
// Increasing rollout from 25 to 50 will add users, never remove existing ones.
func IsRolledOut(userID, flagKey string, rollout int32, salt string) (bool, error) {
	if rollout < 0 || rollout > 100 {
		return false, ErrInvalidRollout
	}
	if rollout == 0 {
		return false, nil // Fast path: disabled for everyone
	}
	if rollout == 100 {
		return true, nil // Fast path: enabled for everyone
	}
	if userID == "" {
		return false, nil // No user context, treat as not rolled out
	}

	bucket := BucketUser(userID, flagKey, salt)
	return bucket < int(rollout), nil
}

// ValidateVariants checks that variant weights sum to exactly 100 and all names are non-empty and unique.
// Returns nil if variants slice is empty (no A/B test configured).
func ValidateVariants(variants []Variant) error {
	if len(variants) == 0 {
		return nil // Empty variants is valid (no A/B test)
	}

	totalWeight := 0
	seenNames := make(map[string]bool)
	
	for _, variant := range variants {
		if variant.Name == "" {
			return errors.New("variant name cannot be empty")
		}
		if seenNames[variant.Name] {
			return errors.New("duplicate variant name: " + variant.Name)
		}
		seenNames[variant.Name] = true
		
		if variant.Weight < 0 || variant.Weight > 100 {
			return errors.New("variant weight must be between 0 and 100")
		}
		totalWeight += variant.Weight
	}
	
	if totalWeight != 100 {
		return ErrInvalidVariantWeights
	}
	return nil
}

// GetVariant determines which A/B test variant a user is assigned to based on their bucket.
//
// Algorithm:
//   1. Hash(userID + flagKey + salt) → bucket (0-99)
//   2. Assign variant based on cumulative weight ranges
//
// Example: variants = [A:50, B:30, C:20]
//   - bucket 0-49  → A
//   - bucket 50-79 → B
//   - bucket 80-99 → C
//
// Returns empty string if:
//   - No variants defined
//   - userID is empty (no user context)
//   - Validation fails
func GetVariant(userID, flagKey string, variants []Variant, salt string) (string, error) {
	if len(variants) == 0 {
		return "", nil
	}
	if err := ValidateVariants(variants); err != nil {
		return "", err
	}
	if userID == "" {
		return "", nil // No user context
	}

	bucket := BucketUser(userID, flagKey, salt)
	if bucket < 0 {
		return "", nil // Invalid bucket
	}

	// Assign variant based on cumulative weights
	cumulativeWeight := 0
	for _, variant := range variants {
		cumulativeWeight += variant.Weight
		if bucket < cumulativeWeight {
			return variant.Name, nil
		}
	}

	// Should never reach here if weights sum to 100
	return variants[len(variants)-1].Name, nil
}

// GetVariantConfig returns the configuration for the variant assigned to the user.
// Returns nil if:
//   - No variants are defined
//   - userID is empty
//   - The assigned variant has no config
//   - Validation fails
func GetVariantConfig(userID, flagKey string, variants []Variant, salt string) (map[string]any, error) {
	variantName, err := GetVariant(userID, flagKey, variants, salt)
	if err != nil || variantName == "" {
		return nil, err
	}

	for _, variant := range variants {
		if variant.Name == variantName {
			return variant.Config, nil
		}
	}
	return nil, nil
}
