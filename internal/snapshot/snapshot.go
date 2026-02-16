// Package snapshot provides an in-memory cache of feature flags with ETag-based versioning.
// The snapshot is thread-safe and updated atomically when flags change in the database.
// It supports real-time updates via Server-Sent Events (SSE) to connected clients.
//
// Snapshot Lifecycle:
//   1. Application Startup:
//      - Load flags from database via store.GetAllFlags()
//      - Build snapshot via BuildFromFlags() or BuildFromRows()
//      - Store globally via Update()
//   2. Runtime Operations:
//      - Reads: Load() returns current snapshot (atomic, thread-safe, O(1))
//      - Writes: Admin flag mutations trigger Update() with new snapshot
//   3. SSE Notifications:
//      - Update() automatically broadcasts ETag to connected SSE clients
//      - Clients receive "update" event and can refresh their cache
//
// Global State Management:
//   This package uses package-level global variables for performance and simplicity:
//   - `current`: atomic pointer to current snapshot (modified only via atomic ops)
//   - `rolloutSalt`: stable secret for deterministic user bucketing (set once at startup)
//
//   Both variables are thread-safe:
//   - `current`: protected by atomic.LoadPointer/StorePointer
//   - `rolloutSalt`: set once at startup, then read-only
//
//   This design avoids per-request synchronization overhead and provides O(1) access.
package snapshot

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"sync/atomic"
	"time"
	"unsafe"

	dbgen "github.com/TimurManjosov/goflagship/internal/db/gen"
	"github.com/TimurManjosov/goflagship/internal/rules"
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
	TargetingRules []rules.Rule `json:"targetingRules,omitempty"`
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
// Validation: If salt is empty, this function logs a critical warning but does NOT panic.
// Empty salt will result in predictable (non-random) hashing behavior which may be
// acceptable for testing but is NOT recommended for production.
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
	if salt == "" {
		log.Printf("[snapshot] CRITICAL: SetRolloutSalt called with empty salt. User bucketing will be predictable (not random). This is unsafe for production.")
	} else {
		log.Printf("[snapshot] rollout salt configured (length=%d)", len(salt))
	}
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
//
// Preconditions:
//   - rows may be nil or empty (produces empty snapshot)
//   - rolloutSalt must be set via SetRolloutSalt before calling (global state dependency)
//
// Postconditions:
//   - Always returns non-nil *Snapshot
//   - Snapshot.Flags is never nil (empty map if rows is empty)
//   - Snapshot.ETag is computed from flags (same flags = same ETag)
//   - Snapshot.UpdatedAt is set to current time (non-deterministic)
//   - Snapshot.RolloutSalt is set to global rolloutSalt value
//
// Edge Cases:
//   - rows is nil: Returns snapshot with empty flags map
//   - rows is empty: Returns snapshot with empty flags map
//   - rows contains duplicate keys: Last row wins (map overwrite)
//   - row.Config is invalid JSON: Config is set to nil, no error returned (see below)
//   - row.Description is null: Converted to empty string
//   - row.Expression is null: Set to nil (no expression)
//
// JSON Unmarshaling:
//   - Config unmarshal errors are silently ignored and never returned from this function.
//   - Invalid JSON results in nil config, not an error or log entry.
//   - This behavior is intentionally lenient to support partial/legacy and occasionally
//     corrupted data already stored in the database without breaking flag loading.
//   - Callers MUST NOT rely on this function to validate configuration JSON; validation
//     and error reporting should be performed at write time or via separate tooling.
//
// Usage:
//   This is used when loading flags directly from PostgreSQL queries.
//   For loading from store.Flag objects, use BuildFromFlags instead.
func BuildFromRows(rows []dbgen.Flag) *Snapshot {
	flagsMap := make(map[string]FlagView, len(rows))
	for _, row := range rows {
		var config map[string]any
		if len(row.Config) > 0 {
			_ = json.Unmarshal(row.Config, &config) // Ignore unmarshal errors, config stays nil
		}
		var targetingRules []rules.Rule
		if len(row.TargetingRules) > 0 {
			if err := json.Unmarshal(row.TargetingRules, &targetingRules); err != nil {
				log.Printf("[snapshot] invalid targeting rules for key=%s: %v", row.Key, err)
			}
		}

		flagsMap[row.Key] = FlagView{
			Key:         row.Key,
			Description: textToString(row.Description),
			Enabled:     row.Enabled,
			Rollout:     row.Rollout,
			Expression:  row.Expression, // Already *string from database
			Config:      config,
			TargetingRules: targetingRules,
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
//
// Preconditions:
//   - flags may be nil or empty (produces empty snapshot)
//   - rolloutSalt must be set via SetRolloutSalt before calling (global state dependency)
//
// Postconditions:
//   - Always returns non-nil *Snapshot
//   - Snapshot.Flags is never nil (empty map if flags is empty)
//   - Snapshot.ETag is computed from flags (deterministic for same flags)
//   - Snapshot.UpdatedAt is set to current time (non-deterministic)
//   - Snapshot.RolloutSalt is set to global rolloutSalt value
//   - All variants are converted from store.Variant to snapshot.Variant
//
// Edge Cases:
//   - flags is nil: Returns snapshot with empty flags map
//   - flags is empty: Returns snapshot with empty flags map
//   - flags contains duplicate keys: Last flag wins (map overwrite)
//   - flag has no variants: Variants field is nil (not empty slice)
//   - flag has empty variants slice: Converted to empty slice
//
// Variant Conversion:
//   - Converts store.Variant to snapshot.Variant (same structure)
//   - Preserves all variant fields (Name, Weight, Config)
//   - Empty variant config remains nil
//
// Usage:
//   This is the primary method for creating snapshots from the store layer.
//   Typically called after store.GetAllFlags() returns results.
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
			TargetingRules: flag.TargetingRules,
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
//
// Preconditions:
//   - flagMap may be nil or empty
//
// Postconditions:
//   - Always returns non-empty string in format: W/"<hex-hash>"
//   - Same flag content produces same ETag (deterministic)
//   - Different flag content produces different ETag (high probability)
//   - Format follows HTTP weak ETag convention (W/ prefix)
//
// Algorithm:
//   1. Serialize flagMap to JSON (deterministic for Go maps in JSON)
//   2. Compute SHA-256 hash of serialized JSON
//   3. Encode hash as hex string
//   4. Wrap in weak ETag format: W/"<hex>"
//
// Edge Cases:
//   - flagMap is nil: Produces ETag for empty map
//   - flagMap is empty: Produces ETag for empty map (same as nil)
//   - JSON marshaling fails: Produces ETag for empty byte slice (should never happen)
//
// ETag Format:
//   The ETag changes whenever flag content changes, enabling efficient cache validation.
//   Weak ETag (W/) indicates semantic equivalence rather than byte-for-byte identity.
//
// Performance:
//   Uses SHA-256 for collision resistance.
//   JSON marshaling is deterministic but may be slow for large flag sets.
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
//   - Logs the update with snapshot details for observability
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
	oldSnapshot := Load()
	storeSnapshot(newSnapshot)
	
	// Log the update for observability
	log.Printf("[snapshot] updated: flags=%d old_etag=%s new_etag=%s",
		len(newSnapshot.Flags), oldSnapshot.ETag, newSnapshot.ETag)
	
	publishUpdate(newSnapshot.ETag) // Notify SSE listeners of the change
}
