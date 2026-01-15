// Package evaluation provides server-side flag evaluation logic.
// It evaluates flags for a given user context, handling expressions, rollouts, and variants.
//
// Testing Guide:
//
// This package is designed for easy unit testing without external dependencies.
// All evaluation functions are pure (no I/O, no global state access within functions).
//
// How to Test EvaluateFlag:
//
//  1. Create test flag using snapshot.FlagView struct
//  2. Create test context with Context{UserID: "test-user", Attributes: {...}}
//  3. Call EvaluateFlag with test salt (use consistent salt for determinism)
//  4. Assert on Result fields (Enabled, Variant, Config)
//
// Example:
//
//	flag := snapshot.FlagView{
//	    Key: "test_flag",
//	    Enabled: true,
//	    Rollout: 50,  // 50% rollout
//	}
//	ctx := Context{UserID: "user-123"}
//	result := EvaluateFlag(flag, ctx, "test-salt")
//	// Assert based on expected behavior
//
// Edge Cases to Test:
//
//   - Empty userID: rollout should fail, expression may still pass
//   - Rollout 0%: should always return disabled
//   - Rollout 100%: should always return enabled
//   - Invalid expression: should return disabled (error handled internally)
//   - No variants: should return flag-level config
//   - Variants with no config: should fall back to flag-level config
//   - Empty salt: evaluation works but reduces hash quality
//
// Testing EvaluateAll:
//
//   - Test with empty flag map
//   - Test with keys filter (subset of flags)
//   - Test with keys that don't exist (should be ignored)
//   - Test that non-deterministic order is acceptable (use map)
//
// No Mocking Required:
//
//   All dependencies (snapshot, rollout, targeting) are tested independently.
//   This package tests integration of those components.
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
//
// Preconditions:
//   - flag must have non-empty Key (required for hashing)
//   - salt should be non-empty (empty salt reduces hash quality but is allowed)
//   - ctx.UserID may be empty (treated as anonymous/unauthenticated user)
//   - ctx.Attributes may be nil (treated as empty map)
//
// Postconditions:
//   - Always returns a Result with Key matching flag.Key
//   - Returns Enabled=false if any evaluation step fails or user doesn't match
//   - Result.Variant is empty string when no variants configured or assignment fails
//   - Result.Config is nil when neither flag nor variant has config
//
// Evaluation order (each step can short-circuit to disabled):
//   1. Check enabled field → if false, return disabled
//   2. Evaluate expression (if present) → if false or error, return disabled
//   3. Check rollout (if <100) → hash user ID to determine inclusion
//      - Special cases: empty userID always excluded, rollout=0 always disabled, rollout=100 always enabled
//   4. Determine variant (if configured) → assign based on user bucket
//   5. Return result with resolved config (variant config > flag config)
//
// Edge Cases:
//   - Empty ctx.UserID: expression may still pass, but rollout check will fail
//   - Empty salt: hashing works but produces less random distribution
//   - flag.Rollout = 0: fast-path returns disabled without hashing
//   - flag.Rollout = 100: fast-path returns enabled without hashing
//   - Invalid expression: treated as evaluation failure, returns disabled
//   - No variants: returns flag-level config
//   - Variant with no config: falls back to flag-level config
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
//
// Preconditions:
//   - flags map may be nil or empty (returns empty slice)
//   - ctx is evaluated for each flag (may have empty UserID)
//   - salt is used for all rollout evaluations
//   - keys is an optional filter (empty means evaluate all)
//
// Postconditions:
//   - Returns slice of Results (never nil, may be empty)
//   - When keys is empty, evaluates all flags in map
//   - When keys is non-empty, only evaluates flags that exist in map
//   - Non-existent keys in filter are silently ignored (no error)
//   - Result order is non-deterministic (map iteration order)
//
// Edge Cases:
//   - Empty flags map: returns empty slice (not nil)
//   - keys contains non-existent flag keys: those keys are skipped
//   - keys is empty: evaluates all flags
//   - flags is nil: returns empty slice
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
//
// Preconditions:
//   - ctx.Attributes may be nil (treated as empty map)
//   - ctx.UserID may be empty string
//
// Postconditions:
//   - Returns non-nil map (never nil, at minimum contains "id" key)
//   - "id" key is always present (set to ctx.UserID, may be empty string)
//   - All attributes from ctx.Attributes are copied
//   - Map is pre-sized to avoid reallocations
//
// Edge Cases:
//   - ctx.Attributes is nil: returns map with only "id" key
//   - ctx.UserID is empty: "id" key is set to empty string
//   - ctx.Attributes has "id" key: original "id" is overwritten by ctx.UserID
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
