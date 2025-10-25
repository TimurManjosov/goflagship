package snapshot

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync/atomic"
	"time"
	"unsafe"

	dbgen "github.com/TimurManjosov/goflagship/internal/db/gen"
	"github.com/jackc/pgx/v5/pgtype"
)

type FlagView struct {
	Key        string                 `json:"key"`
	Description string                `json:"description"`
	Enabled    bool                   `json:"enabled"`
	Rollout    int32                  `json:"rollout"`
	Expression *string                `json:"expression,omitempty"`
	Config     map[string]any         `json:"config,omitempty"`
	Env        string                 `json:"env"`
	UpdatedAt  time.Time              `json:"updatedAt"`
}

type Snapshot struct {
	ETag      string              `json:"etag"`
	Flags     map[string]FlagView `json:"flags"`
	UpdatedAt time.Time           `json:"updatedAt"`
}

var current unsafe.Pointer // *Snapshot

func Load() *Snapshot {
	ptr := atomic.LoadPointer(&current)
	if ptr == nil {
		return &Snapshot{ETag: "", Flags: map[string]FlagView{}, UpdatedAt: time.Now().UTC()}
	}
	return (*Snapshot)(ptr)
}


func textToString(t pgtype.Text) string {
    if t.Valid {
        return t.String
    }
    return ""
}

func store(s *Snapshot) { atomic.StorePointer(&current, unsafe.Pointer(s)) }

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
            Expression:  r.Expression,               // already *string
            Config:      cfg,
            Env:         r.Env,
            UpdatedAt:   r.UpdatedAt.Time,           // <— fixed
        }
    }
    blob, _ := json.Marshal(flags)
    sum := sha256.Sum256(blob)
    etag := `W/"` + hex.EncodeToString(sum[:]) + `"`
    return &Snapshot{ETag: etag, Flags: flags, UpdatedAt: time.Now().UTC()}
}

func Update(s *Snapshot) {
	store(s)
	publishUpdate(s.ETag) // <— notify SSE listeners
}

func nullableString(sqlNull *string) *string { return sqlNull }

