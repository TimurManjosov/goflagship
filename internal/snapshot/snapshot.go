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

// Package-level state:
// This package uses package-level global variables for performance and simplicity.
// These variables are thread-safe and should be initialized once at application startup.
var (
	// current holds an atomic pointer to the current snapshot.
	// Thread-safe: Modified only via atomic operations (atomic.StorePointer/LoadPointer).
	// Initialized: Via Update() function, typically called after database load.
	current unsafe.Pointer // Atomic pointer to current *Snapshot

	// rolloutSalt is a stable secret used for deterministic user bucketing.
	// Thread-safe: Set once at startup via SetRolloutSalt, then read-only.
	// Initialized: Must be set via SetRolloutSalt() before first evaluation.
	// Impact: Changing this value will cause all users to be re-bucketed.
	//
	// Lifecycle:
	//   1. Application startup: SetRolloutSalt(os.Getenv("ROLLOUT_SALT"))
	//   2. Runtime: Value is read but never modified
	//   3. Application shutdown: No cleanup needed (read-only after init)
	rolloutSalt string // Global rollout salt configured at startup
)

// SetRolloutSalt configures the global rollout salt used for deterministic user bucketing.
//
// This MUST be called once at application startup with a stable value before any flag
// evaluations occur. The salt ensures consistent user bucketing across server instances
// and restarts.
//
// Thread-safety: This function is NOT thread-safe. It should only be called once
// during application initialization before any concurrent access begins.
//
// Warning: Changing the salt after users have been bucketed will cause all users to be
// re-bucketed into potentially different rollout groups. This can cause feature visibility
// to change unexpectedly for end users.
//
// Example:
//   func main() {
//       salt := os.Getenv("ROLLOUT_SALT")
//       if salt == "" {
//           log.Fatal("ROLLOUT_SALT must be set")
//       }
//       snapshot.SetRolloutSalt(salt)
//       // ... rest of application setup
//   }
func SetRolloutSalt(salt string) {
	rolloutSalt = salt
}

// Load atomically reads the current snapshot from memory.
//
// Thread-safety: This function is thread-safe and can be called concurrently from
// multiple goroutines. It uses atomic operations to read the snapshot pointer.
//
// Returns:
//   - Current snapshot if one has been stored via Update()
//   - Empty snapshot with current rolloutSalt if no snapshot exists yet
//
// Performance: O(1) atomic pointer load - extremely fast, suitable for hot paths.
//
// Example:
//   snap := snapshot.Load()
//   results := evaluation.EvaluateAll(snap.Flags, ctx, snap.RolloutSalt, keys)
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
//
// Thread-safety: This function is thread-safe and can be called from any goroutine.
// It uses atomic operations to store the new snapshot pointer.
//
// Side effects:
//   - Atomically updates the global 'current' pointer
//   - Notifies all SSE subscribers of the change (publishes ETag)
//
// This is the primary way to update the global snapshot after initialization.
// Typically called:
//   1. At application startup after loading flags from database
//   2. After flag mutations (create, update, delete operations)
//
// Performance: O(1) atomic pointer store + SSE notification
//
// Example:
//   flags, _ := store.GetAllFlags(ctx, env)
//   snap := snapshot.BuildFromFlags(flags)
//   snapshot.Update(snap)  // Makes new snapshot visible globally
func Update(newSnapshot *Snapshot) {
	storeSnapshot(newSnapshot)
	publishUpdate(newSnapshot.ETag) // Notify SSE listeners of the change
}
