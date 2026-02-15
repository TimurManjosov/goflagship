package rules

// Operator represents a comparison operator used in targeting conditions.
type Operator string

// Supported targeting operators (string values for clean JSON serialization).
const (
	OpEq       Operator = "eq"
	OpNeq      Operator = "neq"
	OpContains Operator = "contains"
	OpIn       Operator = "in"
	OpGt       Operator = "gt"
	OpLt       Operator = "lt"
	OpGte      Operator = "gte"
	OpLte      Operator = "lte"
	OpSemVerGt Operator = "semver_gt"
	OpSemVerLt Operator = "semver_lt"
)

// Condition represents a single targeting predicate.
// When multiple conditions belong to one Rule, they are evaluated with AND semantics:
// all conditions must match for the rule to apply.
type Condition struct {
	Property string      `json:"property"`
	Operator Operator    `json:"operator"`
	Value    interface{} `json:"value"`
}

// Rule represents a targeting rule for feature-flag evaluation.
// Conditions are combined with AND semantics.
// Distribution maps variant keys to integer weights that must sum to
// exactly 100 (percent mode) or exactly 10 000 (basis-points mode).
type Rule struct {
	ID           string         `json:"id"`
	Conditions   []Condition    `json:"conditions"`
	Distribution map[string]int `json:"distribution"`
}
