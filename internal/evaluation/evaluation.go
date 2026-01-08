// Package evaluation provides server-side flag evaluation logic.
// It evaluates flags for a given user context, handling expressions, rollouts, and variants.
package evaluation

import (
	"time"

	"github.com/TimurManjosov/goflagship/internal/rollout"
	"github.com/TimurManjosov/goflagship/internal/snapshot"
	"github.com/TimurManjosov/goflagship/internal/targeting"
)

// Context represents user context for flag evaluation.
type Context struct {
	UserID     string         `json:"id"`
	Attributes map[string]any `json:"attributes,omitempty"`
}

// Result represents the evaluation result for a single flag.
type Result struct {
	Key     string         `json:"key"`
	Enabled bool           `json:"enabled"`
	Variant string         `json:"variant,omitempty"`
	Config  map[string]any `json:"config,omitempty"`
}

// EvaluateResponse represents the response from the evaluate endpoint.
type EvaluateResponse struct {
	Flags       []Result  `json:"flags"`
	ETag        string    `json:"etag"`
	EvaluatedAt time.Time `json:"evaluatedAt"`
}

// EvaluateFlag evaluates a single flag for the given context.
// Evaluation order:
// 1. Check enabled field → if false, return disabled
// 2. Evaluate expression (if present) → if false, return disabled
// 3. Check rollout (if <100) → hash user ID to determine inclusion
// 4. Determine variant (if configured)
// 5. Return result with resolved config
func EvaluateFlag(flag snapshot.FlagView, ctx Context, salt string) Result {
	result := Result{
		Key:     flag.Key,
		Enabled: false,
	}

	// Step 1: Check enabled field
	if !flag.Enabled {
		return result
	}

	// Step 2: Evaluate expression (if present)
	if flag.Expression != nil && *flag.Expression != "" {
		// Build targeting context from user attributes
		targetCtx := buildTargetingContext(ctx)

		match, err := targeting.Evaluate(*flag.Expression, targetCtx)
		if err != nil || !match {
			return result
		}
	}

	// Step 3: Check rollout
	if flag.Rollout < 100 {
		isRolledOut, err := rollout.IsRolledOut(ctx.UserID, flag.Key, flag.Rollout, salt)
		if err != nil || !isRolledOut {
			return result
		}
	}

	// Flag is enabled for this user
	result.Enabled = true

	// Step 4: Determine variant and resolve config
	result.Variant, result.Config = resolveVariantAndConfig(flag, ctx.UserID, salt)

	return result
}

// EvaluateAll evaluates all flags for the given context.
// If keys is non-empty, only the specified flags are evaluated.
func EvaluateAll(flags map[string]snapshot.FlagView, ctx Context, salt string, keys []string) []Result {
	// Pre-allocate slice with appropriate capacity to avoid reallocation
	var results []Result
	if len(keys) > 0 {
		// When filtering by keys, allocate for requested keys (some may not exist)
		results = make([]Result, 0, len(keys))
		// Evaluate only specified keys
		for _, key := range keys {
			if flag, exists := flags[key]; exists {
				results = append(results, EvaluateFlag(flag, ctx, salt))
			}
			// Non-existent keys are silently ignored
		}
	} else {
		// When evaluating all flags, allocate exact size needed
		results = make([]Result, 0, len(flags))
		// Evaluate all flags
		for _, flag := range flags {
			results = append(results, EvaluateFlag(flag, ctx, salt))
		}
	}

	return results
}

// buildTargetingContext creates a targeting.UserContext from evaluation context.
func buildTargetingContext(ctx Context) targeting.UserContext {
	// Pre-size map to avoid reallocation (1 for ID + attributes)
	targetCtx := make(targeting.UserContext, len(ctx.Attributes)+1)

	// Add user ID
	if ctx.UserID != "" {
		targetCtx["id"] = ctx.UserID
	}

	// Add all attributes
	for k, v := range ctx.Attributes {
		targetCtx[k] = v
	}

	return targetCtx
}

// convertVariants converts snapshot.Variant to rollout.Variant.
func convertVariants(variants []snapshot.Variant) []rollout.Variant {
	result := make([]rollout.Variant, len(variants))
	for i, v := range variants {
		result[i] = rollout.Variant{
			Name:   v.Name,
			Weight: v.Weight,
			Config: v.Config,
		}
	}
	return result
}

// resolveVariantAndConfig determines the variant (if any) and resolves the appropriate config.
// This centralizes the complex logic of choosing between variant config and flag config.
// 
// Fallback behavior:
//   - Returns ("", flag.Config) when no variants are configured
//   - Returns ("", flag.Config) when variant assignment fails or userID is empty
//   - Returns (variantName, variantConfig) when variant has config
//   - Returns (variantName, flag.Config) when variant exists but has no config
//
// Returns: (variantName, config) where variantName may be empty if no variants are configured
// or if variant assignment fails.
func resolveVariantAndConfig(flag snapshot.FlagView, userID, salt string) (string, map[string]any) {
	// No variants configured - use flag-level config
	if len(flag.Variants) == 0 {
		return "", flag.Config
	}

	// Convert once and reuse for both GetVariant and GetVariantConfig calls
	variants := convertVariants(flag.Variants)
	variantName, err := rollout.GetVariant(userID, flag.Key, variants, salt)
	
	// If variant assignment failed or empty, fall back to flag config
	if err != nil || variantName == "" {
		return "", flag.Config
	}

	// Successfully assigned to a variant - get its config
	// Reusing already-converted variants to avoid duplicate conversion
	variantConfig, err := rollout.GetVariantConfig(userID, flag.Key, variants, salt)
	if err != nil {
		// Error getting variant config - fall back to flag config
		return variantName, flag.Config
	}

	// Return variant config if present, otherwise fall back to flag config
	if variantConfig != nil {
		return variantName, variantConfig
	}
	return variantName, flag.Config
}
