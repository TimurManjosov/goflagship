// Package snapshot provides an in-memory cache of feature flags with ETag-based versioning.
// The snapshot is thread-safe and updated atomically when flags change in the database.
// It supports real-time updates via Server-Sent Events (SSE) to connected clients.
package snapshot

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync/atomic"
	"time"
	"unsafe"

	dbgen "github.com/TimurManjosov/goflagship/internal/db/gen"
	"github.com/TimurManjosov/goflagship/internal/store"
	"github.com/jackc/pgx/v5/pgtype"
)

// Variant represents a variant in an A/B test with its weight and configuration.
// This is a simplified version of store.Variant optimized for JSON serialization.
type Variant struct {
	Name   string         `json:"name"`
	Weight int            `json:"weight"`
	Config map[string]any `json:"config,omitempty"`
}

// FlagView represents a read-optimized view of a feature flag for client consumption.
// It contains all flag attributes including targeting rules, rollout percentage, and variants.
type FlagView struct {
	Key         string         `json:"key"`
	Description string         `json:"description"`
	Enabled     bool           `json:"enabled"`
	Rollout     int32          `json:"rollout"`      // Percentage 0-100
	Expression  *string        `json:"expression,omitempty"` // Targeting expression
	Config      map[string]any `json:"config,omitempty"`
	Variants    []Variant      `json:"variants,omitempty"` // For A/B testing
	Env         string         `json:"env"`
	UpdatedAt   time.Time      `json:"updatedAt"`
}

// Snapshot represents an immutable point-in-time view of all feature flags.
// It includes an ETag for cache validation and optional rollout salt for client-side evaluation.
type Snapshot struct {
	ETag        string              `json:"etag"`                   // SHA-256 hash of flags for cache validation
	Flags       map[string]FlagView `json:"flags"`                  // Map of flag key to flag data
	UpdatedAt   time.Time           `json:"updatedAt"`              // Timestamp of snapshot creation
	RolloutSalt string              `json:"rolloutSalt,omitempty"`  // Salt for deterministic user bucketing
}

var (
	current     unsafe.Pointer // Atomic pointer to current *Snapshot
	rolloutSalt string         // Global rollout salt configured at startup
)

// SetRolloutSalt configures the global rollout salt used for deterministic user bucketing.
// This should be called once at application startup with a stable value.
// Changing the salt will cause all users to be re-bucketed into different rollout groups.
func SetRolloutSalt(salt string) {
	rolloutSalt = salt
}

// Load atomically reads the current snapshot from memory.
// Returns an empty snapshot if no snapshot has been stored yet.
func Load() *Snapshot {
	pointer := atomic.LoadPointer(&current)
	if pointer == nil {
		return &Snapshot{
			ETag:        "",
			Flags:       map[string]FlagView{},
			UpdatedAt:   time.Now().UTC(),
			RolloutSalt: rolloutSalt,
		}
	}
	return (*Snapshot)(pointer)
}

// textToString safely extracts a string from a nullable pgx Text field.
func textToString(text pgtype.Text) string {
	if text.Valid {
		return text.String
	}
	return ""
}

// storeSnapshot atomically updates the current snapshot pointer.
func storeSnapshot(snapshot *Snapshot) {
	atomic.StorePointer(&current, unsafe.Pointer(snapshot))
}

// BuildFromRows constructs a snapshot from database query results.
// This is used when loading flags directly from PostgreSQL queries.
func BuildFromRows(rows []dbgen.Flag) *Snapshot {
	flagsMap := make(map[string]FlagView, len(rows))
	for _, row := range rows {
		var config map[string]any
		if len(row.Config) > 0 {
			_ = json.Unmarshal(row.Config, &config) // Ignore unmarshal errors, config stays nil
		}

		flagsMap[row.Key] = FlagView{
			Key:         row.Key,
			Description: textToString(row.Description),
			Enabled:     row.Enabled,
			Rollout:     row.Rollout,
			Expression:  row.Expression, // Already *string from database
			Config:      config,
			Env:         row.Env,
			UpdatedAt:   row.UpdatedAt.Time,
		}
	}
	
	etag := computeETag(flagsMap)
	return &Snapshot{
		ETag:        etag,
		Flags:       flagsMap,
		UpdatedAt:   time.Now().UTC(),
		RolloutSalt: rolloutSalt,
	}
}

// BuildFromFlags creates a snapshot from store.Flag objects.
// This is the primary method for creating snapshots from the store layer.
func BuildFromFlags(flags []store.Flag) *Snapshot {
	flagMap := make(map[string]FlagView, len(flags))
	for _, flag := range flags {
		// Convert store.Variant to snapshot.Variant
		var variants []Variant
		if len(flag.Variants) > 0 {
			variants = make([]Variant, len(flag.Variants))
			for i, variant := range flag.Variants {
				variants[i] = Variant{
					Name:   variant.Name,
					Weight: variant.Weight,
					Config: variant.Config,
				}
			}
		}
		
		flagMap[flag.Key] = FlagView{
			Key:         flag.Key,
			Description: flag.Description,
			Enabled:     flag.Enabled,
			Rollout:     flag.Rollout,
			Expression:  flag.Expression,
			Config:      flag.Config,
			Variants:    variants,
			Env:         flag.Env,
			UpdatedAt:   flag.UpdatedAt,
		}
	}
	
	etag := computeETag(flagMap)
	return &Snapshot{
		ETag:        etag,
		Flags:       flagMap,
		UpdatedAt:   time.Now().UTC(),
		RolloutSalt: rolloutSalt,
	}
}

// computeETag generates a weak ETag from the flag map using SHA-256.
// The ETag changes whenever flag content changes, enabling efficient cache validation.
func computeETag(flagMap map[string]FlagView) string {
	serialized, _ := json.Marshal(flagMap)
	hash := sha256.Sum256(serialized)
	return `W/"` + hex.EncodeToString(hash[:]) + `"`
}

// Update atomically replaces the current snapshot and notifies SSE listeners.
// This is the only way to update the global snapshot after initialization.
func Update(newSnapshot *Snapshot) {
	storeSnapshot(newSnapshot)
	publishUpdate(newSnapshot.ETag) // Notify SSE listeners of the change
}
