package engine

import (
	"sort"
	"strings"

	"github.com/TimurManjosov/goflagship/internal/rules"
	"github.com/TimurManjosov/goflagship/internal/store"
	"github.com/cespare/xxhash/v2"
)

// Evaluate computes deterministic rule-based evaluation for a flag and user context.
func Evaluate(flag *store.Flag, context *UserContext) EvaluationResult {
	result := EvaluationResult{Variant: defaultVariant}
	if flag == nil {
		result.Reason = string(ReasonDisabled)
		return result
	}

	result.Value = flag.Config

	if !flag.Enabled {
		result.Reason = string(ReasonDisabled)
		return result
	}

	for _, rule := range flag.TargetingRules {
		if !matchesAllConditions(context, rule.Conditions) {
			continue
		}

		result.Variant = selectVariant(flag.Key, context, flag.Config, rule.Distribution)
		result.Value = resolveValue(flag, result.Variant)
		result.Reason = string(ReasonTargetingMatch)
		result.MatchedRule = rule.ID
		return result
	}

	result.Variant = selectVariant(flag.Key, context, flag.Config, defaultDistribution(flag))
	result.Value = resolveValue(flag, result.Variant)
	result.Reason = string(ReasonDefaultRollout)
	return result
}

func matchesAllConditions(ctx *UserContext, conditions []rules.Condition) bool {
	for _, condition := range conditions {
		userValue, ok := getContextValue(ctx, condition.Property)
		if !ok {
			return false
		}
		handler, ok := getOperatorHandler(condition.Operator)
		if !ok || !handler.Check(userValue, condition.Value) {
			return false
		}
	}
	return true
}

func getContextValue(ctx *UserContext, property string) (any, bool) {
	if ctx == nil {
		return nil, false
	}

	switch strings.ToLower(property) {
	case "id", "user_id", "userid":
		if ctx.ID == "" {
			return nil, false
		}
		return ctx.ID, true
	case "email":
		if ctx.Email == "" {
			return nil, false
		}
		return ctx.Email, true
	case "country":
		if ctx.Country == "" {
			return nil, false
		}
		return ctx.Country, true
	case "plan":
		if ctx.Plan == "" {
			return nil, false
		}
		return ctx.Plan, true
	}

	if ctx.Properties == nil {
		return nil, false
	}
	v, ok := ctx.Properties[property]
	return v, ok
}

func selectVariant(flagKey string, ctx *UserContext, config map[string]any, distribution map[string]int) string {
	total := distributionTotal(distribution)
	if total <= 0 {
		return defaultVariant
	}

	bucket := hashBucket(flagKey, ctx, config, total)
	if bucket < 0 {
		return defaultVariant
	}

	keys := make([]string, 0, len(distribution))
	for variant, weight := range distribution {
		if weight <= 0 {
			continue
		}
		keys = append(keys, variant)
	}
	if len(keys) == 0 {
		return defaultVariant
	}
	sort.Strings(keys)

	cumulative := 0
	for _, variant := range keys {
		cumulative += distribution[variant]
		if bucket < cumulative {
			return variant
		}
	}
	return keys[len(keys)-1]
}

func hashBucket(flagKey string, ctx *UserContext, config map[string]any, total int) int {
	if ctx == nil || ctx.ID == "" || total <= 0 {
		return -1
	}
	salt := ""
	if config != nil {
		if rawSalt, ok := config["salt"]; ok {
			s, ok := rawSalt.(string)
			if ok {
				salt = s
			}
		}
	}

	seed := ctx.ID + ":" + flagKey + ":" + salt
	hash := xxhash.Sum64String(seed)
	return int(hash % uint64(total))
}

func distributionTotal(distribution map[string]int) int {
	total := 0
	for _, weight := range distribution {
		if weight > 0 {
			total += weight
		}
	}
	return total
}

func defaultDistribution(flag *store.Flag) map[string]int {
	if len(flag.Variants) == 0 {
		return map[string]int{defaultVariant: 100}
	}
	distribution := make(map[string]int, len(flag.Variants))
	for _, variant := range flag.Variants {
		distribution[variant.Name] = variant.Weight
	}
	return distribution
}

func resolveValue(flag *store.Flag, variant string) any {
	for _, v := range flag.Variants {
		if v.Name == variant && v.Config != nil {
			return v.Config
		}
	}
	return flag.Config
}
