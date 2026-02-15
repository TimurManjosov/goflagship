package engine

import (
	"encoding/json"
	"regexp"
	"strings"
	"sync"

	"github.com/Masterminds/semver/v3"
	"github.com/TimurManjosov/goflagship/internal/rules"
)

// OperatorHandler evaluates one condition operator.
type OperatorHandler interface {
	Check(userValue, ruleValue any) bool
}

const (
	opEquals     rules.Operator = "equals"
	opNotEquals  rules.Operator = "not_equals"
	opContains   rules.Operator = "contains"
	opStartsWith rules.Operator = "starts_with"
	opEndsWith   rules.Operator = "ends_with"
	opRegex      rules.Operator = "regex"
	opGT         rules.Operator = "gt"
	opLT         rules.Operator = "lt"
	opGTE        rules.Operator = "gte"
	opLTE        rules.Operator = "lte"
	opInList     rules.Operator = "in_list"
	opNotInList  rules.Operator = "not_in_list"
	opVersionGT  rules.Operator = "version_gt"
	opVersionLT  rules.Operator = "version_lt"
)

var (
	operatorHandlers = map[rules.Operator]OperatorHandler{
		opEquals:     equalsHandler{},
		opNotEquals:  notEqualsHandler{},
		opContains:   containsHandler{},
		opStartsWith: startsWithHandler{},
		opEndsWith:   endsWithHandler{},
		opRegex:      regexHandler{},
		opGT:         numericCompareHandler{cmp: func(a, b float64) bool { return a > b }},
		opLT:         numericCompareHandler{cmp: func(a, b float64) bool { return a < b }},
		opGTE:        numericCompareHandler{cmp: func(a, b float64) bool { return a >= b }},
		opLTE:        numericCompareHandler{cmp: func(a, b float64) bool { return a <= b }},
		opInList:     inListHandler{},
		opNotInList:  notInListHandler{},
		opVersionGT:  semverCompareHandler{cmp: func(a, b *semver.Version) bool { return a.GreaterThan(b) }},
		opVersionLT:  semverCompareHandler{cmp: func(a, b *semver.Version) bool { return a.LessThan(b) }},
	}
	// regexCache keeps compiled regex by pattern for the hot evaluation path.
	// Expected value type is *regexp.Regexp.
	regexCache sync.Map
)

func getOperatorHandler(op rules.Operator) (OperatorHandler, bool) {
	normalized := normalizeOperator(op)
	h, ok := operatorHandlers[normalized]
	return h, ok
}

func normalizeOperator(op rules.Operator) rules.Operator {
	switch strings.ToLower(string(op)) {
	case "==", "eq", "equals":
		return opEquals
	case "!=", "neq", "not_equals":
		return opNotEquals
	case "contains":
		return opContains
	case "starts_with", "startswith":
		return opStartsWith
	case "ends_with", "endswith":
		return opEndsWith
	case "regex", "matches":
		return opRegex
	case ">", "gt":
		return opGT
	case "<", "lt":
		return opLT
	case ">=", "gte":
		return opGTE
	case "<=", "lte":
		return opLTE
	case "in", "in_list":
		return opInList
	case "not_in", "not_in_list", "nin":
		return opNotInList
	case "semver_gt", "version_gt":
		return opVersionGT
	case "semver_lt", "version_lt":
		return opVersionLT
	default:
		return op
	}
}

type equalsHandler struct{}

func (equalsHandler) Check(userValue, ruleValue any) bool {
	if user, ok := toString(userValue); ok {
		rule, ok := toString(ruleValue)
		return ok && equalsString(user, rule)
	}
	if user, ok := toFloat64(userValue); ok {
		rule, ok := toFloat64(ruleValue)
		return ok && user == rule
	}
	if user, ok := userValue.(bool); ok {
		rule, ok := ruleValue.(bool)
		return ok && user == rule
	}
	return false
}

type notEqualsHandler struct{}

func (notEqualsHandler) Check(userValue, ruleValue any) bool {
	return !equalsHandler{}.Check(userValue, ruleValue)
}

type containsHandler struct{}

func (containsHandler) Check(userValue, ruleValue any) bool {
	user, ok := toString(userValue)
	if !ok {
		return false
	}
	rule, ok := toString(ruleValue)
	if !ok {
		return false
	}
	return strings.Contains(normalizeCase(user), normalizeCase(rule))
}

type startsWithHandler struct{}

func (startsWithHandler) Check(userValue, ruleValue any) bool {
	user, ok := toString(userValue)
	if !ok {
		return false
	}
	rule, ok := toString(ruleValue)
	if !ok {
		return false
	}
	return strings.HasPrefix(normalizeCase(user), normalizeCase(rule))
}

type endsWithHandler struct{}

func (endsWithHandler) Check(userValue, ruleValue any) bool {
	user, ok := toString(userValue)
	if !ok {
		return false
	}
	rule, ok := toString(ruleValue)
	if !ok {
		return false
	}
	return strings.HasSuffix(normalizeCase(user), normalizeCase(rule))
}

type regexHandler struct{}

func (regexHandler) Check(userValue, ruleValue any) bool {
	user, ok := toString(userValue)
	if !ok {
		return false
	}
	pattern, ok := toString(ruleValue)
	if !ok {
		return false
	}

	rx, ok := getCompiledRegex(pattern)
	if !ok {
		return false
	}
	return rx.MatchString(user)
}

type numericCompareHandler struct {
	cmp func(a, b float64) bool
}

func (h numericCompareHandler) Check(userValue, ruleValue any) bool {
	user, ok := toFloat64(userValue)
	if !ok {
		return false
	}
	rule, ok := toFloat64(ruleValue)
	if !ok {
		return false
	}
	return h.cmp(user, rule)
}

type inListHandler struct{}

func (inListHandler) Check(userValue, ruleValue any) bool {
	user, ok := toString(userValue)
	if !ok {
		return false
	}
	list, ok := toStringSlice(ruleValue)
	if !ok {
		return false
	}
	for _, item := range list {
		if equalsString(user, item) {
			return true
		}
	}
	return false
}

type notInListHandler struct{}

func (notInListHandler) Check(userValue, ruleValue any) bool {
	return !inListHandler{}.Check(userValue, ruleValue)
}

type semverCompareHandler struct {
	cmp func(a, b *semver.Version) bool
}

func (h semverCompareHandler) Check(userValue, ruleValue any) bool {
	userStr, ok := toString(userValue)
	if !ok {
		return false
	}
	ruleStr, ok := toString(ruleValue)
	if !ok {
		return false
	}
	userVer, err := semver.NewVersion(userStr)
	if err != nil {
		return false
	}
	ruleVer, err := semver.NewVersion(ruleStr)
	if err != nil {
		return false
	}
	return h.cmp(userVer, ruleVer)
}

func getCompiledRegex(pattern string) (*regexp.Regexp, bool) {
	if cached, ok := regexCache.Load(pattern); ok {
		rx, ok := cached.(*regexp.Regexp)
		return rx, ok
	}

	rx, err := regexp.Compile(pattern)
	if err != nil {
		return nil, false
	}
	regexCache.Store(pattern, rx)
	return rx, true
}

func toString(v any) (string, bool) {
	s, ok := v.(string)
	return s, ok
}

func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

func toStringSlice(v any) ([]string, bool) {
	switch values := v.(type) {
	case []string:
		return values, true
	case []any:
		result := make([]string, 0, len(values))
		for _, item := range values {
			s, ok := item.(string)
			if !ok {
				return nil, false
			}
			result = append(result, s)
		}
		return result, true
	default:
		return nil, false
	}
}

func equalsString(left, right string) bool {
	return normalizeCase(left) == normalizeCase(right)
}

func normalizeCase(value string) string {
	// Keep case policy centralized; current behavior is case-sensitive.
	return value
}
