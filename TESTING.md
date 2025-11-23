# Testing Guide

This document describes the testing practices and guidelines for the goflagship project.

## Running Tests

### Basic Test Commands

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run specific package tests
go test ./internal/snapshot/...
go test ./internal/api/...

# Run specific test
go test -v -run TestSnapshotEndpoint ./internal/api/...
```

### Using Makefile

```bash
make test           # Run all tests
make test-verbose   # Run with verbose output
make test-race      # Run with race detector
make test-cover     # Generate coverage report (creates coverage.html)
make clean          # Clean test cache and coverage files
```

### Coverage Reports

Generate and view coverage reports:

```bash
# Generate coverage profile
go test -coverprofile=coverage.out ./...

# View coverage in terminal
go tool cover -func=coverage.out

# Generate HTML coverage report
go tool cover -html=coverage.out -o coverage.html

# Open in browser (macOS)
open coverage.html

# Or use the Makefile
make test-cover  # Generates and displays coverage.html
```

## Test Organization

### Test Files

- Unit tests live alongside the code they test: `package_test.go` next to `package.go`
- Use the same package name for white-box testing: `package snapshot`
- Use `package snapshot_test` for black-box testing (testing public API only)

### Test Structure

Tests follow the **Arrange-Act-Assert** pattern:

```go
func TestFunctionName(t *testing.T) {
    // Arrange - set up test data and dependencies
    input := setupTestData()
    
    // Act - execute the function being tested
    result := FunctionName(input)
    
    // Assert - verify the results
    if result != expected {
        t.Errorf("Expected %v, got %v", expected, result)
    }
}
```

### Table-Driven Tests

Use table-driven tests for multiple similar test cases:

```go
func TestValidation(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        wantErr bool
    }{
        {"valid input", "test", false},
        {"empty input", "", true},
        {"invalid chars", "test@#", true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := Validate(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

## Test Categories

### Unit Tests

Test individual functions and methods in isolation:

- `internal/snapshot/snapshot_test.go` - Tests snapshot building and ETag generation
- `internal/snapshot/notify_test.go` - Tests pub/sub notification system
- `internal/store/memory_test.go` - Tests in-memory store operations

### Integration Tests

Test complete workflows with real dependencies:

- `internal/api/server_test.go` - Tests HTTP endpoints end-to-end
- `internal/api/sse_test.go` - Tests Server-Sent Events functionality

### Concurrency Tests

Test thread safety and race conditions:

- `internal/api/concurrent_test.go` - Tests concurrent API operations

## Test Utilities

### Test Helpers

Use the `testutil` package for common test setup:

```go
import "github.com/TimurManjosov/goflagship/internal/testutil"

func TestMyHandler(t *testing.T) {
    // Create test server with in-memory store
    srv, store := testutil.NewTestServer(t, "prod", "test-key")
    
    // Seed test data
    testutil.SeedFlags(ctx, store, []store.UpsertParams{
        {Key: "test", Enabled: true, Rollout: 100, Env: "prod"},
    })
    
    // Make test HTTP request
    req := testutil.HTTPRequest{
        Method: "GET",
        Path:   "/v1/flags/snapshot",
    }
    rr := req.Do(t, srv.Router())
}
```

### Cleanup

Always clean up resources in tests:

```go
func TestWithCleanup(t *testing.T) {
    resource := acquireResource()
    defer resource.Close()  // Ensure cleanup
    
    // Or use t.Cleanup
    t.Cleanup(func() {
        resource.Close()
    })
    
    // Test code...
}
```

## Race Detection

Run tests with the race detector to catch data races:

```bash
# Run all tests with race detection
go test -race ./...

# Run specific package
go test -race ./internal/snapshot/...
```

**Note**: Some SSE tests may show false positives with the race detector due to limitations in `httptest.ResponseRecorder` with streaming connections. The application code itself is race-free.

## Continuous Integration

Tests run automatically on every push and pull request via GitHub Actions:

- Tests run on Go 1.21, 1.22, and 1.23
- Race detection runs on snapshot and store packages
- Coverage reports are generated and uploaded as artifacts
- Linting runs via golangci-lint

See `.github/workflows/tests.yml` for the complete CI configuration.

## Best Practices

### DO

- ✅ Write descriptive test names: `TestSnapshotEndpoint_ETag_NotModified`
- ✅ Use subtests for variations: `t.Run("negative rollout", func(t *testing.T) {...})`
- ✅ Test both success and error cases
- ✅ Use `t.Helper()` in test helper functions
- ✅ Keep tests fast (< 100ms for unit tests, < 1s for integration)
- ✅ Make tests hermetic (no external dependencies)
- ✅ Clean up resources with `defer` or `t.Cleanup()`

### DON'T

- ❌ Don't use `time.Sleep()` for synchronization (use channels and timeouts)
- ❌ Don't share state between tests
- ❌ Don't test implementation details, test behavior
- ❌ Don't ignore test failures
- ❌ Don't write flaky tests that pass/fail randomly

## Coverage Goals

Target coverage levels:

- **Snapshot package**: > 80%
- **API package**: > 70%
- **Store package**: > 80%

Check current coverage:

```bash
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | grep total
```

## Debugging Tests

### Verbose Output

```bash
# See detailed test output
go test -v ./internal/api/...

# See which tests run
go test -v -run TestSSE ./internal/api/...
```

### Test-Specific Flags

```bash
# Run only short tests
go test -short ./...

# Set timeout for slow tests
go test -timeout 30s ./...

# Run tests in parallel
go test -parallel 4 ./...
```

### Print Debugging

Use `t.Logf()` instead of `fmt.Printf()` in tests:

```go
func TestDebug(t *testing.T) {
    result := calculate()
    t.Logf("Result: %v", result)  // Only shown if test fails or -v flag
}
```

## Writing New Tests

When adding new features:

1. Write tests first (TDD) or alongside the implementation
2. Cover success cases and error cases
3. Test edge cases (empty input, nil values, boundaries)
4. Add concurrency tests if the code uses locks or atomics
5. Update this guide if introducing new testing patterns

## Additional Resources

- [Go Testing Package](https://pkg.go.dev/testing)
- [Table Driven Tests](https://go.dev/wiki/TableDrivenTests)
- [Go Race Detector](https://go.dev/doc/articles/race_detector)
- [httptest Package](https://pkg.go.dev/net/http/httptest)
