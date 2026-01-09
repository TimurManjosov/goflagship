package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/TimurManjosov/goflagship/internal/store"
	"github.com/jackc/pgx/v5/pgtype"
)

// ===== HTTP Helpers =====

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]any{
		"error":   http.StatusText(code),
		"message": msg,
	})
}

// ===== UUID Helpers =====

// formatUUID formats a pgtype.UUID to a standard UUID string.
// Returns an empty string if the UUID is not valid.
//
// Format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
func formatUUID(uuid pgtype.UUID) string {
	if !uuid.Valid {
		return ""
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		uuid.Bytes[0:4], uuid.Bytes[4:6], uuid.Bytes[6:8], uuid.Bytes[8:10], uuid.Bytes[10:16])
}

// parseUUID parses a UUID string into pgtype.UUID.
// The string must be in the standard UUID format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
//
// Returns an error if the format is invalid.
func parseUUID(s string) (pgtype.UUID, error) {
	var uuid pgtype.UUID
	var b [16]byte
	var parts [5]uint64

	// Parse the UUID string into 5 parts
	_, err := fmt.Sscanf(s, "%8x-%4x-%4x-%4x-%12x",
		&parts[0], &parts[1], &parts[2], &parts[3], &parts[4])
	if err != nil {
		return uuid, fmt.Errorf("invalid UUID format: %w", err)
	}

	// Convert parts to bytes (big-endian)
	b[0] = byte(parts[0] >> 24)
	b[1] = byte(parts[0] >> 16)
	b[2] = byte(parts[0] >> 8)
	b[3] = byte(parts[0])
	
	b[4] = byte(parts[1] >> 8)
	b[5] = byte(parts[1])
	
	b[6] = byte(parts[2] >> 8)
	b[7] = byte(parts[2])
	
	b[8] = byte(parts[3] >> 8)
	b[9] = byte(parts[3])
	
	b[10] = byte(parts[4] >> 40)
	b[11] = byte(parts[4] >> 32)
	b[12] = byte(parts[4] >> 24)
	b[13] = byte(parts[4] >> 16)
	b[14] = byte(parts[4] >> 8)
	b[15] = byte(parts[4])

	uuid.Bytes = b
	uuid.Valid = true
	return uuid, nil
}

// ===== Timestamp Helpers =====

// formatTimestamp formats a pgtype.Timestamptz to RFC3339 string.
// Returns an empty string if the timestamp is not valid.
func formatTimestamp(ts pgtype.Timestamptz) string {
	if !ts.Valid {
		return ""
	}
	return ts.Time.Format(time.RFC3339)
}

// formatOptionalTimestamp formats a pgtype.Timestamptz to an optional RFC3339 string pointer.
// Returns nil if the timestamp is not valid, otherwise returns a pointer to the formatted string.
func formatOptionalTimestamp(ts pgtype.Timestamptz) *string {
	if !ts.Valid {
		return nil
	}
	formatted := ts.Time.Format(time.RFC3339)
	return &formatted
}

// ===== Conversion Helpers =====

// flagToMap converts a store.Flag to a map for audit logging.
// Returns nil if the flag is nil.
func flagToMap(flag *store.Flag) map[string]any {
	if flag == nil {
		return nil
	}

	m := map[string]any{
		"key":         flag.Key,
		"description": flag.Description,
		"enabled":     flag.Enabled,
		"rollout":     flag.Rollout,
		"env":         flag.Env,
		"updated_at":  flag.UpdatedAt.Format(time.RFC3339),
	}

	if flag.Expression != nil {
		m["expression"] = *flag.Expression
	}

	if flag.Config != nil {
		m["config"] = flag.Config
	}

	if len(flag.Variants) > 0 {
		variants := make([]map[string]any, len(flag.Variants))
		for i, v := range flag.Variants {
			variants[i] = map[string]any{
				"name":   v.Name,
				"weight": v.Weight,
			}
			if v.Config != nil {
				variants[i]["config"] = v.Config
			}
		}
		m["variants"] = variants
	}

	return m
}
