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

	// Step 4: Determine variant (if configured)
	if len(flag.Variants) > 0 {
		// Convert snapshot.Variant to rollout.Variant
		variants := convertVariants(flag.Variants)

		variantName, err := rollout.GetVariant(ctx.UserID, flag.Key, variants, salt)
		if err == nil && variantName != "" {
			result.Variant = variantName

			// Get variant-specific config
			variantConfig, _ := rollout.GetVariantConfig(ctx.UserID, flag.Key, variants, salt)
			if variantConfig != nil {
				result.Config = variantConfig
			} else if flag.Config != nil {
				// Fall back to flag-level config if no variant config
				result.Config = flag.Config
			}
		}
	} else if flag.Config != nil {
		// No variants, use flag-level config
		result.Config = flag.Config
	}

	return result
}

// EvaluateAll evaluates all flags for the given context.
// If keys is non-empty, only the specified flags are evaluated.
func EvaluateAll(flags map[string]snapshot.FlagView, ctx Context, salt string, keys []string) []Result {
	results := make([]Result, 0)

	if len(keys) > 0 {
		// Evaluate only specified keys
		for _, key := range keys {
			if flag, exists := flags[key]; exists {
				results = append(results, EvaluateFlag(flag, ctx, salt))
			}
			// Non-existent keys are silently ignored
		}
	} else {
		// Evaluate all flags
		for _, flag := range flags {
			results = append(results, EvaluateFlag(flag, ctx, salt))
		}
	}

	return results
}

// buildTargetingContext creates a targeting.UserContext from evaluation context.
func buildTargetingContext(ctx Context) targeting.UserContext {
	targetCtx := make(targeting.UserContext)

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
