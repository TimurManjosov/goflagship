package store

import (
	"context"
	"encoding/json"
	"errors"

	dbgen "github.com/TimurManjosov/goflagship/internal/db/gen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	emptyJSONObject = "{}"
)

// PostgresStore is a PostgreSQL implementation of the Store interface.
//
// Thread Safety:
//   All methods are safe for concurrent access via pgxpool connection pooling.
//   The underlying connection pool manages concurrency and connection lifecycle.
//
// Error Handling:
//   - Database errors are returned as-is (wrapped pgx errors)
//   - "Not found" errors are converted to domain-specific errors
//   - JSON marshaling errors are returned for invalid config data
//
// Resource Management:
//   - Close() must be called when done to release connections
//   - Close() is safe to call multiple times
//   - After Close(), no methods should be called
//
// Lifecycle:
//   1. Create: NewPostgresStore(pool)
//   2. Use: Call GetAllFlags, UpsertFlag, etc.
//   3. Cleanup: Close() to release resources
type PostgresStore struct {
	pool *pgxpool.Pool
	q    *dbgen.Queries
}

// NewPostgresStore creates a new PostgreSQL-backed store.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{
		pool: pool,
		q:    dbgen.New(pool),
	}
}

// GetAllFlags retrieves all flags for the given environment from the database.
//
// Preconditions:
//   - ctx must be non-nil (panic if nil)
//   - env may be empty string (will return flags for empty env, likely none)
//
// Postconditions:
//   - Returns []Flag (never nil, may be empty) on success
//   - Returns error if database query fails or conversion fails
//   - Does not return nil slice on success (returns empty slice instead)
//
// Edge Cases:
//   - env="": Returns flags with env="" (if any exist in DB)
//   - No matching flags: Returns empty slice (not error)
//   - Database error: Returns nil slice with error
//   - Invalid JSON in config: Returns error (conversion fails)
//
// Performance:
//   Pre-allocates result slice with capacity = query result size.
//   Converts all rows to domain objects before returning.
func (p *PostgresStore) GetAllFlags(ctx context.Context, env string) ([]Flag, error) {
	dbFlags, err := p.q.GetAllFlags(ctx, env)
	if err != nil {
		return nil, err
	}

	flags := make([]Flag, 0, len(dbFlags))
	for _, dbFlag := range dbFlags {
		flag, err := p.convertFromDB(dbFlag)
		if err != nil {
			return nil, err
		}
		flags = append(flags, flag)
	}

	return flags, nil
}

// GetFlagByKey retrieves a single flag by its key from the database.
//
// Preconditions:
//   - ctx must be non-nil (panic if nil)
//   - key may be empty (will likely not find any flag)
//
// Postconditions:
//   - Returns non-nil *Flag on success
//   - Returns error if flag not found or database query fails
//   - Flag not found returns domain error "flag not found" (not pgx.ErrNoRows)
//
// Edge Cases:
//   - key="": Likely returns "flag not found" (unless empty key exists in DB)
//   - Multiple flags with same (key, env): Returns first match (DB/schema should prevent this)
//   - Multiple environments with same key: Behavior depends on underlying query ordering and may be ambiguous
//   - Flag exists but has invalid JSON config: Returns error
//
// Error Types:
//   - "flag not found": Flag doesn't exist in database
//   - Other errors: Database connectivity or data conversion errors
func (p *PostgresStore) GetFlagByKey(ctx context.Context, key string) (*Flag, error) {
	dbFlag, err := p.q.GetFlagByKey(ctx, key)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("flag not found")
		}
		return nil, err
	}

	flag, err := p.convertFromDB(dbFlag)
	if err != nil {
		return nil, err
	}

	return &flag, nil
}

// UpsertFlag creates or updates a flag in the database.
//
// Preconditions:
//   - ctx must be non-nil (panic if nil)
//   - params.Key must be non-empty (database constraint)
//   - params.Env must be non-empty (database constraint)
//   - params.Config may be nil (converted to empty JSON object {})
//
// Postconditions:
//   - Returns nil on success (flag created or updated)
//   - Returns error if database constraint violated or operation fails
//   - If flag exists (key+env match), it's updated
//   - If flag doesn't exist, it's created
//
// Idempotent:
//   Calling with same params multiple times produces same result.
//   Safe to retry on transient failures.
//
// Edge Cases:
//   - params.Config=nil: Stored as {} (empty JSON object)
//   - params.Config={}: Stored as {} (empty JSON object)
//   - params.Description="": Stored as empty string
//   - params.Expression=nil: Stored as NULL in database
//   - Invalid JSON in Config: Returns JSON marshaling error
//   - Key/Env combination exists: Updates existing flag
//
// Database Constraints:
//   Primary key: (key, env) - ensures uniqueness per environment
func (p *PostgresStore) UpsertFlag(ctx context.Context, params UpsertParams) error {
	// Convert config map to JSON bytes
	var configBytes []byte
	if params.Config != nil {
		b, err := json.Marshal(params.Config)
		if err != nil {
			return err
		}
		configBytes = b
	} else {
		configBytes = []byte(emptyJSONObject)
	}

	dbParams := dbgen.UpsertFlagParams{
		Key:         params.Key,
		Description: pgtype.Text{String: params.Description, Valid: true},
		Enabled:     params.Enabled,
		Rollout:     params.Rollout,
		Expression:  params.Expression,
		Config:      configBytes,
		Env:         params.Env,
	}

	return p.q.UpsertFlag(ctx, dbParams)
}

// DeleteFlag removes a flag from the database.
//
// Preconditions:
//   - ctx must be non-nil (panic if nil)
//   - key and env identify the flag to delete
//
// Postconditions:
//   - Returns nil on success (flag deleted or didn't exist)
//   - Idempotent: deleting non-existent flag is not an error
//
// Edge Cases:
//   - Flag doesn't exist: Returns nil (not an error, idempotent)
//   - key="": Attempts to delete flag with empty key (likely no-op)
//   - env="": Attempts to delete flag with empty env (likely no-op)
//   - Database error: Returns error
//
// Idempotent:
//   Safe to call multiple times with same parameters.
//   Deleting a flag that doesn't exist is considered success.
func (p *PostgresStore) DeleteFlag(ctx context.Context, key, env string) error {
	return p.q.DeleteFlag(ctx, dbgen.DeleteFlagParams{
		Key: key,
		Env: env,
	})
}

// Close closes the database connection pool.
//
// Preconditions:
//   - None (safe to call anytime)
//
// Postconditions:
//   - Pool is closed and resources released
//   - No database operations should be performed after Close
//   - Safe to call multiple times (subsequent calls are no-ops)
//
// Thread Safety:
//   Safe to call from multiple goroutines (pgxpool handles this).
//
// Edge Cases:
//   - Pool already closed: No error, no-op
//   - Pending queries: May be canceled or completed (pool dependent)
//
// Always returns nil (implements io.Closer interface convention).
func (p *PostgresStore) Close() error {
	p.pool.Close()
	return nil
}

// --- API Keys ---

// ListAPIKeys retrieves all API keys from the database
func (p *PostgresStore) ListAPIKeys(ctx context.Context) ([]dbgen.ApiKey, error) {
	return p.q.ListAPIKeys(ctx)
}

// CreateAPIKey creates a new API key in the database
func (p *PostgresStore) CreateAPIKey(ctx context.Context, params dbgen.CreateAPIKeyParams) (dbgen.ApiKey, error) {
	return p.q.CreateAPIKey(ctx, params)
}

// GetAPIKeyByID retrieves an API key by its ID
func (p *PostgresStore) GetAPIKeyByID(ctx context.Context, id pgtype.UUID) (dbgen.ApiKey, error) {
	return p.q.GetAPIKeyByID(ctx, id)
}

// UpdateAPIKeyLastUsed updates the last_used_at timestamp for an API key
func (p *PostgresStore) UpdateAPIKeyLastUsed(ctx context.Context, id pgtype.UUID) error {
	return p.q.UpdateAPIKeyLastUsed(ctx, id)
}

// RevokeAPIKey disables an API key
func (p *PostgresStore) RevokeAPIKey(ctx context.Context, id pgtype.UUID) error {
	return p.q.RevokeAPIKey(ctx, id)
}

// DeleteAPIKey permanently deletes an API key
func (p *PostgresStore) DeleteAPIKey(ctx context.Context, id pgtype.UUID) error {
	return p.q.DeleteAPIKey(ctx, id)
}

// --- Audit Logs ---

// CreateAuditLog creates a new audit log entry
func (p *PostgresStore) CreateAuditLog(ctx context.Context, params dbgen.CreateAuditLogParams) error {
	return p.q.CreateAuditLog(ctx, params)
}

// ListAuditLogs retrieves audit logs with pagination and filtering
func (p *PostgresStore) ListAuditLogs(ctx context.Context, params dbgen.ListAuditLogsParams) ([]dbgen.AuditLog, error) {
	return p.q.ListAuditLogs(ctx, params)
}

// CountAuditLogs returns the total count of audit logs with filtering
func (p *PostgresStore) CountAuditLogs(ctx context.Context, params dbgen.CountAuditLogsParams) (int64, error) {
	return p.q.CountAuditLogs(ctx, params)
}

// GetAuditLogsByAPIKey retrieves audit logs for a specific API key
func (p *PostgresStore) GetAuditLogsByAPIKey(ctx context.Context, apiKeyID pgtype.UUID, limit, offset int32) ([]dbgen.AuditLog, error) {
	return p.q.GetAuditLogsByAPIKey(ctx, dbgen.GetAuditLogsByAPIKeyParams{
		ApiKeyID: apiKeyID,
		Limit:    limit,
		Offset:   offset,
	})
}

// GetQueries returns the underlying sqlc Queries for direct access
func (p *PostgresStore) GetQueries() *dbgen.Queries {
	return p.q
}

// convertFromDB converts a database Flag to a store Flag.
func (p *PostgresStore) convertFromDB(dbFlag dbgen.Flag) (Flag, error) {
	var config map[string]any
	if len(dbFlag.Config) > 0 {
		if err := json.Unmarshal(dbFlag.Config, &config); err != nil {
			return Flag{}, err
		}
	}

	description := ""
	if dbFlag.Description.Valid {
		description = dbFlag.Description.String
	}

	return Flag{
		Key:         dbFlag.Key,
		Description: description,
		Enabled:     dbFlag.Enabled,
		Rollout:     dbFlag.Rollout,
		Expression:  dbFlag.Expression,
		Config:      config,
		Env:         dbFlag.Env,
		UpdatedAt:   dbFlag.UpdatedAt.Time,
	}, nil
}
