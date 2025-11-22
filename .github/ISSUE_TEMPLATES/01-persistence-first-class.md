# Unify Store Interface for Memory and Postgres Persistence

## Problem / Motivation

Currently, the persistence layer is tightly coupled to PostgreSQL with an in-memory snapshot acting as a read cache. While this works, it lacks:

1. **Abstraction**: No clean `Store` interface that can swap between memory-only and Postgres implementations
2. **Parity**: Memory and Postgres don't have feature parity (e.g., memory store doesn't support all operations)
3. **Efficiency**: Snapshot rebuild reads all flags from DB on every mutation, which doesn't scale well
4. **Flexibility**: Hard to add other backends (Redis, SQLite, etc.) or run tests without Postgres

This makes the system harder to test, deploy in different configurations, and maintain.

## Proposed Solution

Introduce a unified `Store` interface that abstracts all flag persistence operations. Provide two implementations:

1. **MemoryStore**: Pure in-memory store for development/testing (no external dependencies)
2. **PostgresStore**: Production-ready Postgres-backed store with efficient snapshot rebuilds

Both implementations should:
- Satisfy the same interface
- Support all CRUD operations (get, upsert, delete, list)
- Handle environment filtering
- Provide atomic snapshot rebuilds
- Be thread-safe

## Concrete Tasks

### Phase 1: Design Store Interface
- [ ] Define `Store` interface in `internal/store/store.go` with methods:
  - `GetAllFlags(ctx, env) ([]Flag, error)`
  - `GetFlagByKey(ctx, key) (*Flag, error)`
  - `UpsertFlag(ctx, params) error`
  - `DeleteFlag(ctx, key, env) error`
  - `Close() error`
- [ ] Define common `Flag` struct that both implementations use
- [ ] Document expected behavior (atomicity, consistency guarantees)

### Phase 2: Implement MemoryStore
- [ ] Create `internal/store/memory.go` with in-memory map storage
- [ ] Use `sync.RWMutex` for thread-safe access
- [ ] Implement all Store interface methods
- [ ] Add unit tests for concurrent access patterns

### Phase 3: Implement PostgresStore
- [ ] Create `internal/store/postgres.go` wrapping existing repo layer
- [ ] Implement efficient snapshot rebuild (avoid full table scan when possible)
- [ ] Consider using Postgres NOTIFY/LISTEN for change detection
- [ ] Maintain backward compatibility with existing sqlc queries
- [ ] Add integration tests with test database

### Phase 4: Integration
- [ ] Update `internal/api/server.go` to use Store interface
- [ ] Add factory function to create store based on config
- [ ] Update `cmd/server/main.go` to initialize appropriate store
- [ ] Ensure snapshot package works with both store types
- [ ] Update documentation and examples

### Phase 5: Optimization
- [ ] Add incremental snapshot updates (don't rebuild entire snapshot on single flag change)
- [ ] Add caching layer for frequently accessed flags
- [ ] Benchmark memory vs postgres performance
- [ ] Document performance characteristics

## API Changes

### New Interface (internal/store/store.go)
```go
type Store interface {
    GetAllFlags(ctx context.Context, env string) ([]Flag, error)
    GetFlagByKey(ctx context.Context, key string) (*Flag, error)
    UpsertFlag(ctx context.Context, params UpsertParams) error
    DeleteFlag(ctx context.Context, key, env string) error
    Close() error
}

type Flag struct {
    Key         string
    Description string
    Enabled     bool
    Rollout     int32
    Expression  *string
    Config      map[string]any
    Env         string
    UpdatedAt   time.Time
}
```

### Configuration Changes
```bash
# .env additions
STORE_TYPE=postgres  # or "memory"
```

## Acceptance Criteria

- [ ] `Store` interface is defined with comprehensive documentation
- [ ] `MemoryStore` implementation passes all unit tests
- [ ] `PostgresStore` implementation passes all integration tests
- [ ] Both stores can be used interchangeably via config flag
- [ ] Existing API behavior is unchanged (backward compatible)
- [ ] Snapshot rebuild is at least as efficient as current implementation
- [ ] All existing tests pass with both store implementations
- [ ] Performance benchmarks show no regression
- [ ] Documentation updated with:
  - Store interface usage examples
  - How to switch between implementations
  - Performance characteristics of each

## Notes / Risks / Edge Cases

### Risks
- **Breaking Change Risk**: Refactoring persistence layer could break existing functionality
  - Mitigation: Keep PostgresStore as default, maintain backward compatibility
- **Performance Regression**: Abstraction layer might add overhead
  - Mitigation: Benchmark both implementations, optimize hot paths
- **Migration Complexity**: Existing deployments need smooth upgrade path
  - Mitigation: Make PostgresStore default, no config change needed

### Edge Cases
- Concurrent flag updates in MemoryStore need proper locking
- Snapshot consistency during rapid flag mutations
- Transaction handling in PostgresStore for atomic operations
- Handling DB connection failures gracefully
- Memory store persistence across restarts (consider optional file backing)

### Future Enhancements
- Add Redis/Valkey implementation for distributed caching
- Add SQLite implementation for single-binary deployments
- Support composite stores (e.g., write to Postgres, read from Redis)
- Add store-level metrics and tracing

## Implementation Hints

- Current persistence code is in `internal/repo/repo.go` (wraps sqlc)
- Snapshot logic is in `internal/snapshot/snapshot.go` (atomic pointer swap)
- Server uses snapshot in `internal/api/server.go`'s `RebuildSnapshot` method
- Database queries are in `internal/db/queries/flags.sql` (sqlc)
- Consider using `sync/atomic` for snapshot updates in MemoryStore
- Look at how `internal/api/server.go` currently calls `s.repo.*` methods

## Labels

`feature`, `backend`, `refactor`, `good-first-issue` (for MemoryStore unit tests), `performance`

## Estimated Effort

**2-3 days** (experienced Go developer)
- Day 1: Interface design + MemoryStore implementation
- Day 2: PostgresStore refactor + integration
- Day 3: Testing, optimization, documentation
