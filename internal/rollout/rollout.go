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
// Preconditions:
//   - rollout must be in range [0, 100] (returns error otherwise)
//   - userID, flagKey, salt may be empty strings (see edge cases)
//
// Postconditions:
//   - Returns (false, ErrInvalidRollout) if rollout < 0 or > 100
//   - Returns (true/false, nil) for valid rollout values
//   - Never returns true with a non-nil error
//
// Algorithm:
//   1. Hash(userID + flagKey + salt) → bucket (0-99)
//   2. If bucket < rollout percentage, user is included
//
// Edge Cases:
//   - rollout=0: Always returns (false, nil) — flag disabled for all users (fast path)
//   - rollout=100: Always returns (true, nil) — flag enabled for all users (fast path)
//   - rollout<0 or >100: Returns (false, ErrInvalidRollout) — invalid configuration
//   - userID="": Always returns (false, nil) — anonymous users not targeted
//   - flagKey="": Proceeds with empty key (valid but unusual, reduces hash distribution)
//   - salt="": Proceeds with empty salt (valid but not recommended, reduces hash quality)
//
// Deterministic Behavior:
//   Same (userID, flagKey, rollout, salt) always produces same result.
//   Increasing rollout from 25% to 50% adds users, never removes existing ones.
//
// Example: rollout=25 means ~25% of users see the feature.
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
//
// Preconditions:
//   - variants slice may be nil or empty
//
// Postconditions:
//   - Returns nil if variants is empty (no A/B test configured)
//   - Returns error if validation fails
//   - Does not modify the variants slice
//
// Validation Rules:
//   1. Empty variants slice is valid (returns nil)
//   2. All variant names must be non-empty strings
//   3. All variant names must be unique
//   4. All variant weights must be in range [0, 100]
//   5. Sum of all variant weights must equal exactly 100
//
// Edge Cases:
//   - variants=nil: Returns nil (valid, no variants)
//   - variants=[]: Returns nil (valid, no variants)
//   - variants with empty name: Returns error
//   - variants with duplicate names: Returns error
//   - variants with weights summing to 99 or 101: Returns ErrInvalidVariantWeights
//   - single variant with weight=100: Valid (A/B test with 100% control)
//
// Returns nil if valid, or an error describing why it's invalid.
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
// Preconditions:
//   - variants slice should be pre-validated (use ValidateVariants first)
//   - userID, flagKey, salt may be empty strings (see edge cases)
//
// Postconditions:
//   - Returns ("", nil) if no variants or validation fails
//   - Returns (variantName, nil) on successful assignment
//   - Returns ("", error) if validation fails
//   - Never returns non-empty variant name with non-nil error
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
// Edge Cases:
//   - variants=nil or []: Returns ("", nil) — no A/B test configured
//   - userID="": Returns ("", nil) — anonymous users not assigned variants
//   - flagKey="": Proceeds with empty key (unusual but valid)
//   - salt="": Proceeds with empty salt (valid but reduces hash quality)
//   - Validation fails: Returns ("", error) — invalid variant configuration
//   - bucket >= 100: Impossible (BucketUser returns [0, 99]), but handled as last-variant fallback for safety
//
// Deterministic Behavior:
//   Same (userID, flagKey, variants, salt) always produces same variant assignment.
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
//
// Preconditions:
//   - variants slice should be pre-validated
//   - userID, flagKey, salt may be empty
//
// Postconditions:
//   - Returns nil if no variant assigned or variant has no config
//   - Returns config map if variant is assigned and has config
//   - Returns (nil, error) if GetVariant fails
//
// Edge Cases:
//   - No variants defined: Returns (nil, nil)
//   - userID="": Returns (nil, nil) — no user context
//   - Assigned variant has no config: Returns (nil, nil)
//   - Assigned variant has empty config map: Returns empty map (not nil)
//   - GetVariant fails: Returns (nil, error)
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
