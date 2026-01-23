# Build and Run Guide

This document provides canonical commands for building, testing, and running the goflagship application.
These commands are deterministic and suitable for both local development and CI/CD pipelines.

---

## Prerequisites

- **Go 1.25+** (verify with `go version`)
- **PostgreSQL** (optional, only if using postgres store)
- **Make** (optional, for convenience commands)

---

## Quick Start

### 1. Build the Server

```bash
# Using Go directly (canonical)
go build -o bin/server ./cmd/server

# Or using Makefile
make build
```

### 2. Build the CLI Tool

```bash
# Using Go directly (canonical)
go build -o bin/flagship ./cmd/flagship

# Or using Makefile
make build-cli
```

### 3. Run Tests

```bash
# Run all tests (canonical)
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests with race detector
go test -race ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# Or using Makefile
make test              # Run all tests
make test-verbose      # Run with -v flag
make test-race         # Run with race detector
make test-cover        # Generate coverage report
```

---

## Configuration

The application is configured via environment variables. Configuration can be provided in three ways:

1. **Environment variables** (highest priority)
2. **`.env` file** in the repository root (optional)
3. **Default values** (suitable for development only)

### Required Configuration

The following configuration must be set for production:

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `ROLLOUT_SALT` | **Yes** | _(random)_ | Stable salt for deterministic user bucketing. **Must be set explicitly in production.** |
| `DB_DSN` | **Yes** (if postgres) | `postgres://...` | PostgreSQL connection string (required when `STORE_TYPE=postgres`) |
| `ADMIN_API_KEY` | **Yes** (in prod) | `admin-123` | Admin API key. **Must not use default in production.** |

### Optional Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `APP_ENV` | `dev` | Environment name: `dev`, `staging`, `prod` |
| `APP_HTTP_ADDR` | `:8080` | API server bind address |
| `METRICS_ADDR` | `:9090` | Metrics/pprof server bind address |
| `STORE_TYPE` | `postgres` | Storage backend: `memory` or `postgres` |
| `ENV` | `prod` | Flag environment to operate on |

### Example `.env` File

```bash
# Copy from .env.example
cp .env.example .env

# Edit .env with your values
APP_ENV=dev
APP_HTTP_ADDR=:8080
METRICS_ADDR=:9090
DB_DSN=postgres://flagship:flagship@localhost:5432/flagship?sslmode=disable
ADMIN_API_KEY=your-secure-admin-key
STORE_TYPE=postgres
ROLLOUT_SALT=your-stable-production-salt
```

---

## Running the Server

### Development Mode (In-Memory Store)

No database required. Data is lost on restart.

```bash
# Set minimal configuration
export STORE_TYPE=memory
export ROLLOUT_SALT=dev-salt-$(date +%s)
export ADMIN_API_KEY=dev-admin-key

# Run server
go run ./cmd/server

# Or use the built binary
./bin/server
```

### Production Mode (PostgreSQL Store)

Requires PostgreSQL database with migrations applied.

```bash
# Set production configuration
export APP_ENV=prod
export STORE_TYPE=postgres
export DB_DSN="postgres://user:password@localhost:5432/flagship?sslmode=disable"
export ROLLOUT_SALT="your-stable-salt-here"  # DO NOT CHANGE after deployment
export ADMIN_API_KEY="your-secure-key-here"   # DO NOT use default

# Run server
./bin/server
```

### Verify Server is Running

```bash
# Health check
curl http://localhost:8080/healthz

# Metrics (on metrics port)
curl http://localhost:9090/metrics

# Get flag snapshot
curl http://localhost:8080/v1/flags/snapshot
```

---

## Configuration Validation

The server performs strict configuration validation at startup and will fail fast with clear error messages if misconfigured.

### Common Startup Errors

**Error:** `configuration validation failed [ROLLOUT_SALT]: rollout salt cannot be empty`

**Solution:** Set the `ROLLOUT_SALT` environment variable to a stable random string.

```bash
# Generate a secure salt (save this value!)
export ROLLOUT_SALT=$(openssl rand -hex 16)
echo "ROLLOUT_SALT=$ROLLOUT_SALT" >> .env
```

---

**Error:** `configuration validation failed [DB_DSN]: database DSN is required when STORE_TYPE=postgres`

**Solution:** Set the `DB_DSN` environment variable or change to in-memory store.

```bash
export DB_DSN="postgres://user:password@host:port/database"
# Or use in-memory store for development
export STORE_TYPE=memory
```

---

**Error:** `configuration validation failed [ADMIN_API_KEY]: default admin API key 'admin-123' is not allowed in production`

**Solution:** Set a secure admin API key when `APP_ENV=prod`.

```bash
export ADMIN_API_KEY="$(openssl rand -base64 32)"
```

---

**Error:** `failed to initialize store: failed to create postgres pool: invalid database DSN`

**Solution:** Check your `DB_DSN` format. It should follow the pattern:
```
postgres://username:password@host:port/database?sslmode=disable
```

---

## CI/CD Pipeline Commands

The following commands are deterministic and suitable for CI/CD:

### Full Build and Test Pipeline

```bash
# 1. Download dependencies
go mod download

# 2. Run linters (if golangci-lint is installed)
golangci-lint run --timeout=5m

# 3. Run tests
go test ./...

# 4. Run tests with race detector (subset of packages)
go test -race ./internal/snapshot/... ./internal/store/...

# 5. Build server binary
go build -v ./cmd/server

# 6. Build CLI binary
go build -v ./cmd/flagship

# 7. Verify binaries were created
test -f ./server && test -f ./flagship
```

### Minimal CI Check

```bash
go test ./... && go build -v ./cmd/server
```

---

## Development Workflow

### 1. Make Code Changes

Edit files in your preferred editor.

### 2. Run Affected Tests

```bash
# Test specific package
go test ./internal/config/...

# Test with verbose output
go test -v ./internal/api/...
```

### 3. Run Full Test Suite

```bash
make test
```

### 4. Build and Run

```bash
make build
STORE_TYPE=memory ROLLOUT_SALT=dev-salt ./bin/server
```

### 5. Clean Build Artifacts

```bash
make clean
```

---

## Troubleshooting

### Tests Fail in CI but Pass Locally

**Common causes:**
- Time zone differences (tests using time.Now())
- File system differences (case sensitivity)
- Race conditions (run `go test -race ./...` locally)

**Solution:** Ensure tests use deterministic inputs and mock time-dependent code.

---

### Server Exits Immediately

**Check configuration validation:**

```bash
# Run with explicit config to see validation errors
STORE_TYPE=postgres DB_DSN="" ./bin/server
```

The server will print a clear error message indicating what configuration is missing or invalid.

---

### Database Connection Fails

**Verify PostgreSQL is running:**

```bash
psql -h localhost -U flagship -d flagship
```

**Check DSN format:**
```bash
# Valid format
postgres://username:password@host:port/database?sslmode=disable

# Common mistakes
postgres://host:port/database  # Missing username/password
postgresql://...               # Use 'postgres://' not 'postgresql://'
```

---

## Summary

| Task | Command |
|------|---------|
| Build server | `go build -o bin/server ./cmd/server` |
| Build CLI | `go build -o bin/flagship ./cmd/flagship` |
| Run tests | `go test ./...` |
| Run server (dev) | `STORE_TYPE=memory ROLLOUT_SALT=dev ./bin/server` |
| Run server (prod) | Set all required env vars, then `./bin/server` |
| Clean artifacts | `make clean` |

For more details, see:
- [TESTING.md](TESTING.md) - Testing practices and guidelines
- [CONTRIBUTING.md](CONTRIBUTING.md) - Contribution guidelines
- [.env.example](.env.example) - Example configuration file
