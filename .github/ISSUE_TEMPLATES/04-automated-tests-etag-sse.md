# Add Automated Tests for ETag and SSE Semantics

## Problem / Motivation

The current codebase lacks comprehensive automated tests, particularly for the critical real-time synchronization features:

1. **No Unit Tests**: Core business logic (snapshot, validation) is untested
2. **No Integration Tests**: End-to-end flows (flag creation → SSE notification) are untested
3. **ETag Semantics**: No tests verify proper ETag generation, matching, or 304 responses
4. **SSE Behavior**: No tests for SSE connection lifecycle, event delivery, or reconnection
5. **Concurrency**: No tests for race conditions during concurrent flag updates

This creates risks:
- Regressions go undetected
- Refactoring is risky
- New contributors lack confidence
- Bug fixes can't be verified systematically

## Proposed Solution

Build a comprehensive test suite covering:

1. **Unit Tests**: Test individual components in isolation
2. **Integration Tests**: Test API endpoints with real HTTP requests
3. **SSE Tests**: Test Server-Sent Events connection and event delivery
4. **ETag Tests**: Verify ETag generation, caching, and conditional requests
5. **Concurrency Tests**: Test race conditions and thread safety

## Concrete Tasks

### Phase 1: Test Infrastructure Setup
- [ ] Create `internal/api/server_test.go` for API tests
- [ ] Create `internal/snapshot/snapshot_test.go` for snapshot tests
- [ ] Set up test helpers in `internal/testutil/`:
  - `NewTestServer()` - creates server with test config
  - `NewTestRepo()` - creates in-memory repo for tests
  - `HTTPRequest()` - helper for making test HTTP requests
- [ ] Add test database setup/teardown helpers
- [ ] Configure `go test` to run with race detector (`-race` flag)
- [ ] Add `make test` target to Makefile (or document in README)

### Phase 2: Unit Tests for Snapshot Package
- [ ] Test `BuildFromRows()`:
  - Empty rows → empty snapshot
  - Multiple flags → correct map construction
  - ETag generation is deterministic
  - Config JSON unmarshaling
- [ ] Test `Load()` and `Update()`:
  - Load empty snapshot returns default
  - Update swaps pointer atomically
- [ ] Test `Subscribe()` and `Unsubscribe()`:
  - Multiple subscribers receive updates
  - Unsubscribed clients stop receiving
  - Closed channel doesn't panic
- [ ] Test concurrent access (using goroutines)

### Phase 3: Unit Tests for Validation Logic
- [ ] Test flag key validation:
  - Valid keys: `feature_x`, `banner-msg`, `test123`
  - Invalid keys: empty, spaces, special chars, too long
- [ ] Test rollout validation (0-100 range)
- [ ] Test config JSON validation
- [ ] Test environment name validation

### Phase 4: Integration Tests for ETag Semantics
- [ ] Test ETag generation:
  - Same flags → same ETag
  - Different flags → different ETag
  - Flag update → ETag changes
- [ ] Test snapshot endpoint with ETag:
  ```go
  // First request: get ETag
  resp1 := GET("/v1/flags/snapshot")
  etag := resp1.Header.Get("ETag")
  assert.NotEmpty(t, etag)
  
  // Second request with If-None-Match
  resp2 := GET("/v1/flags/snapshot", 
    Header("If-None-Match", etag))
  assert.Equal(t, 304, resp2.StatusCode)
  ```
- [ ] Test cache headers:
  - `Cache-Control: no-cache, no-store, must-revalidate`
  - `Pragma: no-cache`
  - `Expires: 0`
- [ ] Test ETag after flag mutation:
  - Create flag → new ETag
  - Update flag → ETag changes
  - Delete flag → ETag changes

### Phase 5: Integration Tests for SSE
- [ ] Test SSE connection:
  - Headers: `Content-Type: text/event-stream`, `Connection: keep-alive`
  - Initial `init` event with current ETag
  - Connection stays open
- [ ] Test SSE events on flag changes:
  ```go
  client := NewSSEClient("/v1/flags/stream")
  events := client.Events()
  
  // Should receive init event
  initEvent := <-events
  assert.Equal(t, "init", initEvent.Type)
  
  // Update flag in another goroutine
  go UpdateFlag("banner", true)
  
  // Should receive update event
  updateEvent := <-events
  assert.Equal(t, "update", updateEvent.Type)
  assert.NotEmpty(t, updateEvent.Data["etag"])
  ```
- [ ] Test SSE heartbeat (ping every 25s)
- [ ] Test SSE client disconnect (context cancellation)
- [ ] Test multiple SSE clients receive same events
- [ ] Test SSE reconnection behavior

### Phase 6: API Endpoint Tests
- [ ] Test `POST /v1/flags`:
  - Valid request → 200 + new ETag
  - Invalid JSON → 400
  - Missing key → 400
  - Invalid rollout → 400
  - Unauthorized (no token) → 401
  - Forbidden (invalid token) → 403
- [ ] Test `DELETE /v1/flags`:
  - Valid request → 200
  - Missing key → 400
  - Missing env → 400
  - Non-existent flag → 200 (idempotent)
  - Unauthorized → 401
- [ ] Test `GET /v1/flags/snapshot`:
  - Returns correct JSON structure
  - Contains all flags for environment
  - Filters by environment correctly

### Phase 7: Concurrency and Race Tests
- [ ] Test concurrent flag updates:
  ```go
  var wg sync.WaitGroup
  for i := 0; i < 100; i++ {
    wg.Add(1)
    go func(n int) {
      defer wg.Done()
      UpdateFlag(fmt.Sprintf("flag_%d", n), true)
    }(i)
  }
  wg.Wait()
  // Verify all flags created
  ```
- [ ] Test concurrent snapshot reads during updates
- [ ] Test concurrent SSE subscriptions
- [ ] Run tests with `-race` flag to detect data races

### Phase 8: Test Documentation
- [ ] Document how to run tests in README:
  ```bash
  # Run all tests
  go test ./...
  
  # Run with race detector
  go test -race ./...
  
  # Run specific package
  go test ./internal/api
  
  # Run with coverage
  go test -cover ./...
  ```
- [ ] Add test coverage report generation:
  ```bash
  go test -coverprofile=coverage.out ./...
  go tool cover -html=coverage.out -o coverage.html
  ```
- [ ] Document testing best practices
- [ ] Add CI/CD pipeline configuration (GitHub Actions)

## API Changes

No API changes required. This is purely testing infrastructure.

## Acceptance Criteria

### Test Coverage
- [ ] Unit test coverage: >80% for `internal/snapshot`
- [ ] Unit test coverage: >70% for `internal/api`
- [ ] Integration tests cover all HTTP endpoints
- [ ] ETag tests verify:
  - Deterministic generation
  - Proper 304 responses
  - ETag changes on mutations
- [ ] SSE tests verify:
  - Connection establishment
  - Event delivery
  - Multiple clients
  - Disconnection handling
- [ ] All tests pass with `-race` flag (no data races)

### Test Quality
- [ ] Tests are hermetic (no external dependencies)
- [ ] Tests use table-driven style where appropriate
- [ ] Tests have clear arrange-act-assert structure
- [ ] Tests clean up resources (defer statements)
- [ ] Tests use meaningful assertions with good error messages
- [ ] Tests are fast (<100ms per test for unit, <1s for integration)

### CI/CD
- [ ] GitHub Actions workflow runs tests on every PR
- [ ] Tests run on multiple Go versions (1.21, 1.22)
- [ ] Coverage report is generated and visible
- [ ] Tests must pass before merge

## Notes / Risks / Edge Cases

### Testing Challenges
- **SSE Testing**: Requires long-lived connections and goroutines
  - Mitigation: Use channels and timeouts to prevent hanging tests
- **Race Detection**: `-race` flag can make tests slower
  - Mitigation: Run race tests separately in CI, not locally by default
- **Database State**: Integration tests need clean state between runs
  - Mitigation: Use transactions that roll back, or use in-memory store
- **Timing Issues**: SSE event delivery isn't instantaneous
  - Mitigation: Use `time.After()` with reasonable timeouts, not `time.Sleep()`

### Edge Cases to Test
- ETag with empty snapshot (no flags)
- ETag with identical flags in different order (should be same)
- SSE client connects then immediately disconnects
- SSE multiple rapid flag updates (coalescing?)
- Snapshot read during rebuild
- Flag delete of non-existent flag (should succeed)
- Concurrent delete of same flag

### Test Organization
- Keep unit tests in same package as code (`package snapshot`)
- Integration tests can use `package api_test` for black-box testing
- Use subtests (`t.Run()`) for table-driven tests
- Use test fixtures in `testdata/` directory for complex JSON

### Future Test Enhancements
- Load testing (simulate 1000s of SSE connections)
- Chaos testing (random failures, timeouts)
- Property-based testing (using `gopter` or similar)
- End-to-end tests with real browser (Playwright)
- Performance benchmarks (`go test -bench`)
- Mutation testing (ensure tests actually catch bugs)

## Implementation Hints

- Use `httptest.NewServer()` for integration tests
- Use `httptest.NewRecorder()` for handler unit tests
- SSE client testing: parse `text/event-stream` format manually or use library
- Example SSE test client:
  ```go
  type SSEClient struct {
    events chan Event
    ctx    context.Context
    cancel context.CancelFunc
  }
  
  func (c *SSEClient) Connect(url string) {
    req, _ := http.NewRequestWithContext(c.ctx, "GET", url, nil)
    resp, _ := http.DefaultClient.Do(req)
    reader := bufio.NewReader(resp.Body)
    // Parse SSE format: "event: init\ndata: {...}\n\n"
  }
  ```
- Use `testify/assert` or `testify/require` for cleaner assertions
- Current code structure:
  - Handlers in `internal/api/server.go`
  - Snapshot in `internal/snapshot/`
  - Repo in `internal/repo/`

## Labels

`feature`, `backend`, `testing`, `good-first-issue` (for simple unit tests), `help-wanted`

## Estimated Effort

**2-3 days**
- Day 1: Test infrastructure + snapshot unit tests + validation tests
- Day 2: Integration tests for API + ETag tests
- Day 3: SSE tests + concurrency tests + CI/CD setup + documentation
