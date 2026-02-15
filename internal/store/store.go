package store

import (
	"context"
	"time"

	"github.com/TimurManjosov/goflagship/internal/rules"
)

// Store defines the interface for flag persistence operations.
// Implementations must be thread-safe and support concurrent access.
type Store interface {
	// GetAllFlags retrieves all flags for the given environment.
	// Returns an empty slice if no flags are found.
	GetAllFlags(ctx context.Context, env string) ([]Flag, error)

	// GetFlagByKey retrieves a single flag by its key.
	// Returns an error if the flag is not found.
	GetFlagByKey(ctx context.Context, key string) (*Flag, error)

	// UpsertFlag creates or updates a flag.
	// If a flag with the same key exists, it will be updated.
	UpsertFlag(ctx context.Context, params UpsertParams) error

	// DeleteFlag removes a flag by key and environment.
	// Returns no error if the flag doesn't exist (idempotent).
	DeleteFlag(ctx context.Context, key, env string) error

	// Close releases any resources held by the store.
	// After Close is called, the store should not be used.
	Close() error
}

// Variant represents a variant in an A/B test or multi-variant experiment.
type Variant struct {
	Name   string         `json:"name"`
	Weight int            `json:"weight"`           // Percentage weight (0-100)
	Config map[string]any `json:"config,omitempty"` // Optional config for this variant
}

// Flag represents a feature flag with all its attributes.
type Flag struct {
	Key            string         `json:"key"`
	Description    string         `json:"description"`
	Enabled        bool           `json:"enabled"`
	Rollout        int32          `json:"rollout"`
	Expression     *string        `json:"expression,omitempty"`
	Config         map[string]any `json:"config,omitempty"`
	TargetingRules []rules.Rule   `json:"targetingRules"`
	Variants       []Variant      `json:"variants,omitempty"` // For A/B testing
	Env            string         `json:"env"`
	UpdatedAt      time.Time      `json:"updatedAt"`
}

// UpsertParams contains the parameters for upserting a flag.
type UpsertParams struct {
	Key            string         `json:"key"`
	Description    string         `json:"description"`
	Enabled        bool           `json:"enabled"`
	Rollout        int32          `json:"rollout"`
	Expression     *string        `json:"expression,omitempty"`
	Config         map[string]any `json:"config,omitempty"`
	TargetingRules []rules.Rule   `json:"targetingRules"`
	Variants       []Variant      `json:"variants,omitempty"` // For A/B testing
	Env            string         `json:"env"`
}
