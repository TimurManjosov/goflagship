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

// Variant represents a variant in an A/B test (mirrored from store for JSON)
type Variant struct {
	Name   string         `json:"name"`
	Weight int            `json:"weight"`
	Config map[string]any `json:"config,omitempty"`
}

type FlagView struct {
	Key         string         `json:"key"`
	Description string         `json:"description"`
	Enabled     bool           `json:"enabled"`
	Rollout     int32          `json:"rollout"`
	Expression  *string        `json:"expression,omitempty"`
	Config      map[string]any `json:"config,omitempty"`
	Variants    []Variant      `json:"variants,omitempty"` // For A/B testing
	Env         string         `json:"env"`
	UpdatedAt   time.Time      `json:"updatedAt"`
}

type Snapshot struct {
	ETag       string              `json:"etag"`
	Flags      map[string]FlagView `json:"flags"`
	UpdatedAt  time.Time           `json:"updatedAt"`
	RolloutSalt string             `json:"rolloutSalt,omitempty"` // Salt for client-side rollout evaluation
}

var current unsafe.Pointer // *Snapshot
var rolloutSalt string     // Global rollout salt

// SetRolloutSalt sets the global rollout salt for the snapshot
func SetRolloutSalt(salt string) {
	rolloutSalt = salt
}

func Load() *Snapshot {
	ptr := atomic.LoadPointer(&current)
	if ptr == nil {
		return &Snapshot{ETag: "", Flags: map[string]FlagView{}, UpdatedAt: time.Now().UTC(), RolloutSalt: rolloutSalt}
	}
	return (*Snapshot)(ptr)
}

func textToString(t pgtype.Text) string {
	if t.Valid {
		return t.String
	}
	return ""
}

func storeSnapshot(s *Snapshot) { atomic.StorePointer(&current, unsafe.Pointer(s)) }

func BuildFromRows(rows []dbgen.Flag) *Snapshot {
	flags := make(map[string]FlagView, len(rows))
	for _, r := range rows {
		var cfg map[string]any
		if len(r.Config) > 0 {
			_ = json.Unmarshal(r.Config, &cfg) // r.Config is []byte if you kept that override
		}

		flags[r.Key] = FlagView{
			Key:         r.Key,
			Description: textToString(r.Description), // <— fixed
			Enabled:     r.Enabled,
			Rollout:     r.Rollout,
			Expression:  r.Expression, // already *string
			Config:      cfg,
			Env:         r.Env,
			UpdatedAt:   r.UpdatedAt.Time, // <— fixed
		}
	}
	blob, _ := json.Marshal(flags)
	sum := sha256.Sum256(blob)
	etag := `W/"` + hex.EncodeToString(sum[:]) + `"`
	return &Snapshot{ETag: etag, Flags: flags, UpdatedAt: time.Now().UTC(), RolloutSalt: rolloutSalt}
}

// BuildFromFlags creates a snapshot from store.Flag objects.
func BuildFromFlags(flags []store.Flag) *Snapshot {
	flagMap := make(map[string]FlagView, len(flags))
	for _, f := range flags {
		// Convert store.Variant to snapshot.Variant
		var variants []Variant
		if len(f.Variants) > 0 {
			variants = make([]Variant, len(f.Variants))
			for i, v := range f.Variants {
				variants[i] = Variant{
					Name:   v.Name,
					Weight: v.Weight,
					Config: v.Config,
				}
			}
		}
		flagMap[f.Key] = FlagView{
			Key:         f.Key,
			Description: f.Description,
			Enabled:     f.Enabled,
			Rollout:     f.Rollout,
			Expression:  f.Expression,
			Config:      f.Config,
			Variants:    variants,
			Env:         f.Env,
			UpdatedAt:   f.UpdatedAt,
		}
	}
	blob, _ := json.Marshal(flagMap)
	sum := sha256.Sum256(blob)
	etag := `W/"` + hex.EncodeToString(sum[:]) + `"`
	return &Snapshot{ETag: etag, Flags: flagMap, UpdatedAt: time.Now().UTC(), RolloutSalt: rolloutSalt}
}

func Update(s *Snapshot) {
	storeSnapshot(s)
	publishUpdate(s.ETag) // <— notify SSE listeners
}

func nullableString(sqlNull *string) *string { return sqlNull }
