// Package rollout provides deterministic user bucketing for feature flag rollouts.
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

// IsRolledOut returns true if the user is within the rollout percentage.
// For rollout=0, always returns false. For rollout=100, always returns true.
// Empty userID returns false (no user context means no rollout).
func IsRolledOut(userID, flagKey string, rollout int32, salt string) (bool, error) {
	if rollout < 0 || rollout > 100 {
		return false, ErrInvalidRollout
	}
	if rollout == 0 {
		return false, nil
	}
	if rollout == 100 {
		return true, nil
	}
	if userID == "" {
		return false, nil // No user context, treat as not rolled out
	}

	bucket := BucketUser(userID, flagKey, salt)
	return bucket < int(rollout), nil
}

// ValidateVariants checks that variant weights sum to 100 and all names are non-empty.
func ValidateVariants(variants []Variant) error {
	if len(variants) == 0 {
		return nil // Empty variants is valid (no A/B test)
	}

	totalWeight := 0
	seenNames := make(map[string]bool)
	for _, v := range variants {
		if v.Name == "" {
			return errors.New("variant name cannot be empty")
		}
		if seenNames[v.Name] {
			return errors.New("duplicate variant name: " + v.Name)
		}
		seenNames[v.Name] = true
		if v.Weight < 0 || v.Weight > 100 {
			return errors.New("variant weight must be between 0 and 100")
		}
		totalWeight += v.Weight
	}
	if totalWeight != 100 {
		return ErrInvalidVariantWeights
	}
	return nil
}

// GetVariant returns the variant name for the given user and flag based on weights.
// Returns empty string if no variants are defined or if userID is empty.
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
	cumulative := 0
	for _, v := range variants {
		cumulative += v.Weight
		if bucket < cumulative {
			return v.Name, nil
		}
	}

	// Should never reach here if weights sum to 100
	return variants[len(variants)-1].Name, nil
}

// GetVariantConfig returns the config for the variant assigned to the user.
// Returns nil if no variants, userID is empty, or variant has no config.
func GetVariantConfig(userID, flagKey string, variants []Variant, salt string) (map[string]any, error) {
	variant, err := GetVariant(userID, flagKey, variants, salt)
	if err != nil || variant == "" {
		return nil, err
	}

	for _, v := range variants {
		if v.Name == variant {
			return v.Config, nil
		}
	}
	return nil, nil
}
