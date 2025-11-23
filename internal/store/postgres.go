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
// It wraps the existing sqlc-generated queries for database operations.
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
func (p *PostgresStore) DeleteFlag(ctx context.Context, key, env string) error {
	return p.q.DeleteFlag(ctx, dbgen.DeleteFlagParams{
		Key: key,
		Env: env,
	})
}

// Close closes the database connection pool.
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
func (p *PostgresStore) CreateAuditLog(ctx context.Context, apiKeyID pgtype.UUID, action, resource, ipAddress, userAgent string, status int32, details map[string]interface{}) error {
	detailsJSON, err := json.Marshal(details)
	if err != nil {
		detailsJSON = []byte("{}")
	}

	return p.q.CreateAuditLog(ctx, dbgen.CreateAuditLogParams{
		ApiKeyID:  apiKeyID,
		Action:    action,
		Resource:  resource,
		IpAddress: ipAddress,
		UserAgent: userAgent,
		Status:    status,
		Details:   detailsJSON,
	})
}

// ListAuditLogs retrieves audit logs with pagination
func (p *PostgresStore) ListAuditLogs(ctx context.Context, limit, offset int32) ([]dbgen.AuditLog, error) {
	return p.q.ListAuditLogs(ctx, dbgen.ListAuditLogsParams{
		Limit:  limit,
		Offset: offset,
	})
}

// CountAuditLogs returns the total count of audit logs
func (p *PostgresStore) CountAuditLogs(ctx context.Context) (int64, error) {
	return p.q.CountAuditLogs(ctx)
}

// GetAuditLogsByAPIKey retrieves audit logs for a specific API key
func (p *PostgresStore) GetAuditLogsByAPIKey(ctx context.Context, apiKeyID pgtype.UUID, limit, offset int32) ([]dbgen.AuditLog, error) {
	return p.q.GetAuditLogsByAPIKey(ctx, dbgen.GetAuditLogsByAPIKeyParams{
		ApiKeyID: apiKeyID,
		Limit:    limit,
		Offset:   offset,
	})
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
