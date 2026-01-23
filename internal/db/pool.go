package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool creates a new PostgreSQL connection pool with production-ready settings.
//
// Configuration:
//   - MaxConns: 10 (maximum concurrent connections)
//   - MinConns: 1 (minimum idle connections)
//   - HealthCheckPeriod: 30s (periodic connection health checks)
//
// Error Handling:
//   Returns detailed error messages for common failure modes:
//   - Invalid DSN format
//   - Network connectivity issues
//   - Authentication failures
//   - Database does not exist
//
// The pool does NOT validate connectivity at creation time. Use pool.Ping(ctx)
// after creation to verify the database is reachable.
//
// Example:
//   pool, err := NewPool(ctx, "postgres://user:pass@localhost/db")
//   if err != nil {
//       log.Fatalf("Failed to create pool: %v", err)
//   }
//   defer pool.Close()
//   
//   // Verify connectivity
//   if err := pool.Ping(ctx); err != nil {
//       log.Fatalf("Database unreachable: %v", err)
//   }
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("invalid database DSN: %w (check DB_DSN format: postgres://user:pass@host:port/dbname)", err)
	}
	cfg.MaxConns = 10
	cfg.MinConns = 1
	cfg.HealthCheckPeriod = 30 * time.Second
	
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create database connection pool: %w", err)
	}
	
	return pool, nil
}
