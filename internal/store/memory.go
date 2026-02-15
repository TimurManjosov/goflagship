package store

import (
	"context"
	"errors"
	"sync"
	"time"
)

// MemoryStore is an in-memory implementation of the Store interface.
// It uses a map for storage and RWMutex for thread-safe concurrent access.
// This implementation is suitable for development, testing, or single-instance deployments.
type MemoryStore struct {
	mu    sync.RWMutex
	flags map[string]Flag // key -> Flag
}

// NewMemoryStore creates a new in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		flags: make(map[string]Flag),
	}
}

// GetAllFlags retrieves all flags for the given environment.
func (m *MemoryStore) GetAllFlags(ctx context.Context, env string) ([]Flag, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Preallocate with reasonable capacity estimate
	result := make([]Flag, 0, len(m.flags)/2)
	for _, flag := range m.flags {
		if flag.Env == env {
			result = append(result, flag)
		}
	}
	return result, nil
}

// GetFlagByKey retrieves a single flag by its key.
func (m *MemoryStore) GetFlagByKey(ctx context.Context, key string) (*Flag, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	flag, exists := m.flags[key]
	if !exists {
		return nil, errors.New("flag not found")
	}

	return &flag, nil
}

// UpsertFlag creates or updates a flag in memory.
func (m *MemoryStore) UpsertFlag(ctx context.Context, params UpsertParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	flag := Flag{
		Key:            params.Key,
		Description:    params.Description,
		Enabled:        params.Enabled,
		Rollout:        params.Rollout,
		Expression:     params.Expression,
		Config:         params.Config,
		TargetingRules: ensureRulesInitialized(params.TargetingRules),
		Variants:       params.Variants,
		Env:            params.Env,
		UpdatedAt:      time.Now().UTC(),
	}

	m.flags[params.Key] = flag
	return nil
}

// DeleteFlag removes a flag from memory.
func (m *MemoryStore) DeleteFlag(ctx context.Context, key, env string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if flag exists and matches the environment before deleting
	if flag, exists := m.flags[key]; exists && flag.Env == env {
		delete(m.flags, key)
	}

	// Idempotent: no error if flag doesn't exist
	return nil
}

// Close is a no-op for MemoryStore as there are no resources to release.
func (m *MemoryStore) Close() error {
	return nil
}
