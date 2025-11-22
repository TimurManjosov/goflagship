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
		configBytes = []byte("{}")
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
