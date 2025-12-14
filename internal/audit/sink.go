package audit

import (
	"context"
	"encoding/json"
	"fmt"

	dbgen "github.com/TimurManjosov/goflagship/internal/db/gen"
	"github.com/jackc/pgx/v5/pgtype"
)

// PostgresSink implements AuditSink for PostgreSQL storage
type PostgresSink struct {
	queries *dbgen.Queries
}

// NewPostgresSink creates a new PostgreSQL audit sink
func NewPostgresSink(queries *dbgen.Queries) *PostgresSink {
	return &PostgresSink{queries: queries}
}

// Write persists an audit event to the database
func (s *PostgresSink) Write(ctx context.Context, event AuditEvent) error {
	// Convert event to database parameters
	params := dbgen.CreateAuditLogParams{
		Action:    event.Action,
		IpAddress: event.Source.IPAddress,
		UserAgent: event.Source.UserAgent,
		Status:    httpStatusFromString(event.Status),
	}
	
	// Set optional text fields
	if event.Actor.ID != nil {
		// Try to parse as UUID for api_key_id
		if uuid, err := parseUUIDString(*event.Actor.ID); err == nil {
			params.ApiKeyID = uuid
		}
	}
	
	if event.Actor.Email != nil {
		params.UserEmail = pgtype.Text{String: *event.Actor.Email, Valid: true}
	}
	
	if event.ResourceType != "" {
		params.ResourceType = pgtype.Text{String: event.ResourceType, Valid: true}
	}
	
	if event.ResourceID != "" {
		params.ResourceID = pgtype.Text{String: event.ResourceID, Valid: true}
	}
	
	if event.ProjectID != nil {
		params.ProjectID = pgtype.Text{String: *event.ProjectID, Valid: true}
	}
	
	if event.Environment != nil {
		params.Environment = pgtype.Text{String: *event.Environment, Valid: true}
	}
	
	if event.RequestID != "" {
		params.RequestID = pgtype.Text{String: event.RequestID, Valid: true}
	}
	
	if event.ErrorMessage != nil {
		params.ErrorMessage = pgtype.Text{String: *event.ErrorMessage, Valid: true}
	}
	
	// Legacy resource field - use resource_type/resource_id concatenation
	if event.ResourceType != "" && event.ResourceID != "" {
		resource := event.ResourceType + "/" + event.ResourceID
		params.Resource = pgtype.Text{String: resource, Valid: true}
	}
	
	// Serialize JSON fields
	if event.BeforeState != nil {
		if b, err := json.Marshal(event.BeforeState); err == nil {
			params.BeforeState = b
		}
	}
	
	if event.AfterState != nil {
		if b, err := json.Marshal(event.AfterState); err == nil {
			params.AfterState = b
		}
	}
	
	if event.Changes != nil {
		if b, err := json.Marshal(event.Changes); err == nil {
			params.Changes = b
		}
	}
	
	// Serialize details (metadata)
	details := map[string]any{
		"actor": event.Actor,
	}
	if b, err := json.Marshal(details); err == nil {
		params.Details = b
	} else {
		params.Details = []byte("{}")
	}
	
	return s.queries.CreateAuditLog(ctx, params)
}

// httpStatusFromString converts status string to HTTP status code
func httpStatusFromString(status string) int32 {
	switch status {
	case StatusSuccess:
		return 200
	case StatusFailure:
		return 500
	default:
		return 200
	}
}

// parseUUIDString parses a UUID string into pgtype.UUID
func parseUUIDString(s string) (pgtype.UUID, error) {
	var uuid pgtype.UUID
	var b [16]byte
	
	// Parse UUID format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	var parts [5]uint64
	_, err := parseUUID(s, &parts)
	if err != nil {
		return uuid, err
	}
	
	// Convert parts to bytes
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

// parseUUID is a helper that scans UUID parts
func parseUUID(s string, parts *[5]uint64) (int, error) {
	var dummy int
	n, err := fmt.Sscanf(s, "%8x-%4x-%4x-%4x-%12x%c",
		&parts[0], &parts[1], &parts[2], &parts[3], &parts[4], &dummy)
	if err != nil && n < 5 {
		return n, err
	}
	return n, nil
}
