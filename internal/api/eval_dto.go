package api

// EvaluationRequest is the request payload for POST /v1/evaluate.
type EvaluationRequest struct {
	Context EvaluationContextDTO `json:"context"`
	FlagKey string               `json:"flagKey,omitempty"`
}

// EvaluationContextDTO represents API-layer evaluation context.
type EvaluationContextDTO struct {
	ID         string         `json:"id,omitempty"`
	Email      string         `json:"email,omitempty"`
	Country    string         `json:"country,omitempty"`
	Properties map[string]any `json:"properties,omitempty"`
}

// EvaluationResponse is the response payload for POST /v1/evaluate.
type EvaluationResponse struct {
	Results []FlagResult `json:"results"`
}

// FlagResult represents one evaluated flag result.
type FlagResult struct {
	Key     string `json:"key"`
	Enabled bool   `json:"enabled"`
	Value   any    `json:"value,omitempty"`
	Variant string `json:"variant,omitempty"`
	Reason  string `json:"reason,omitempty"`
}
