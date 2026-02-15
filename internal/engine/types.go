package engine

// Reason represents evaluation result reason.
type Reason string

const (
	ReasonDisabled       Reason = "DISABLED"
	ReasonTargetingMatch Reason = "TARGETING_MATCH"
	ReasonDefaultRollout Reason = "DEFAULT_ROLLOUT"

	defaultVariant = "control"
)

// UserContext contains fixed user attributes and optional custom properties.
type UserContext struct {
	ID         string         `json:"id"`
	Email      string         `json:"email,omitempty"`
	Country    string         `json:"country,omitempty"`
	Plan       string         `json:"plan,omitempty"`
	Properties map[string]any `json:"properties,omitempty"`
}

// EvaluationResult is the deterministic output of Evaluate.
type EvaluationResult struct {
	Value       any    `json:"value"`
	Variant     string `json:"variant"`
	Reason      string `json:"reason"`
	MatchedRule string `json:"matchedRule,omitempty"`
}
