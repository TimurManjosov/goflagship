package store

import (
	"context"
	"fmt"

	mydb "github.com/TimurManjosov/goflagship/internal/db"
)

// NewStore creates a new store based on the given store type.
//
// Supported Types:
//   - "memory": In-memory store (data lost on restart, suitable for development/testing)
//   - "postgres": PostgreSQL-backed store (persistent, suitable for production)
//
// For postgres stores:
//   - Validates that dbDSN is non-empty
//   - Creates connection pool (validates DSN format)
//   - Does NOT verify database connectivity (pool creation is lazy)
//   - Caller should verify connectivity separately if needed
//
// Error Cases:
//   - Unknown storeType: Returns descriptive error listing valid types
//   - Empty dbDSN for postgres: Returns error indicating DSN is required
//   - Invalid postgres DSN: Returns error from pool creation with context
//
// Example:
//   store, err := NewStore(ctx, "postgres", os.Getenv("DB_DSN"))
//   if err != nil {
//       log.Fatalf("Store initialization failed: %v", err)
//   }
//   defer store.Close()
func NewStore(ctx context.Context, storeType, dbDSN string) (Store, error) {
	switch storeType {
	case "memory":
		return NewMemoryStore(), nil
	case "postgres":
		if dbDSN == "" {
			return nil, fmt.Errorf("database DSN cannot be empty when using postgres store (set DB_DSN environment variable)")
		}
		pool, err := mydb.NewPool(ctx, dbDSN)
		if err != nil {
			return nil, fmt.Errorf("failed to create postgres pool: %w", err)
		}
		return NewPostgresStore(pool), nil
	default:
		return nil, fmt.Errorf("unsupported store type: %s (must be 'memory' or 'postgres')", storeType)
	}
}
