package store

import (
	"context"
	"fmt"

	mydb "github.com/TimurManjosov/goflagship/internal/db"
)

// NewStore creates a new store based on the given store type.
// Supported types: "memory", "postgres"
func NewStore(ctx context.Context, storeType, dbDSN string) (Store, error) {
	switch storeType {
	case "memory":
		return NewMemoryStore(), nil
	case "postgres":
		pool, err := mydb.NewPool(ctx, dbDSN)
		if err != nil {
			return nil, fmt.Errorf("failed to create postgres pool: %w", err)
		}
		return NewPostgresStore(pool), nil
	default:
		return nil, fmt.Errorf("unsupported store type: %s", storeType)
	}
}
