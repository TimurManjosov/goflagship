# Contributing to goflagship

Thank you for your interest in contributing to **goflagship**! This document provides guidelines and instructions for contributing to the project.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Workflow](#development-workflow)
- [Code Standards](#code-standards)
- [Testing Guidelines](#testing-guidelines)
- [Pull Request Process](#pull-request-process)
- [Commit Message Conventions](#commit-message-conventions)
- [Reporting Bugs](#reporting-bugs)
- [Suggesting Features](#suggesting-features)

---

## Code of Conduct

This project follows a professional code of conduct. Please be respectful, constructive, and collaborative in all interactions.

---

## Getting Started

### Prerequisites

- **Go 1.25+** - [Download here](https://go.dev/dl/)
- **PostgreSQL 15+** (for database-backed mode)
- **Git** for version control
- **Make** (optional, for convenience commands)

### Setting Up Your Development Environment

1. **Fork the repository** on GitHub

2. **Clone your fork:**
   ```bash
   git clone https://github.com/YOUR_USERNAME/goflagship.git
   cd goflagship
   ```

3. **Add upstream remote:**
   ```bash
   git remote add upstream https://github.com/TimurManjosov/goflagship.git
   ```

4. **Install dependencies:**
   ```bash
   go mod download
   ```

5. **Set up environment variables:**
   ```bash
   cp .env.example .env
   # Edit .env with your local settings
   ```

6. **Run database migrations (if using PostgreSQL):**
   ```bash
   # Install goose
   go install github.com/pressly/goose/v3/cmd/goose@latest
   
   # Run migrations
   goose -dir internal/db/migrations postgres "postgres://user:pass@localhost:5432/flagship?sslmode=disable" up
   ```

7. **Verify your setup:**
   ```bash
   make test
   go run ./cmd/server
   ```

---

## Development Workflow

### Creating a Feature Branch

```bash
# Update your main branch
git checkout main
git pull upstream main

# Create a feature branch
git checkout -b feature/your-feature-name
```

### Running the Application Locally

**Start the server:**
```bash
go run ./cmd/server
# Or use: make run
```

**Build the CLI:**
```bash
go build -o bin/flagship ./cmd/flagship
# Or use: make build-cli
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with race detector (recommended)
go test -race ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Using Make
make test           # Run all tests
make test-race      # Run with race detector
make test-cover     # Generate coverage report
```

### Linting and Formatting

```bash
# Format code
go fmt ./...

# Run goimports (if installed)
goimports -w .

# Run golangci-lint (if installed)
golangci-lint run
```

---

## Code Standards

### Five Clean Code Rules

This project follows five core Clean Code principles:

1. **Intention-Revealing Names** - Use descriptive names that clearly convey purpose
2. **Small Functions with Single Responsibility** - Keep functions focused and under 30 lines
3. **Clear Error Handling** - Always handle errors explicitly with contextual messages
4. **Consistent Formatting and Structure** - Follow Go conventions consistently
5. **Meaningful Comments and Documentation** - Document exported items and complex logic

See [REFACTOR_DAY_REPORT.md](REFACTOR_DAY_REPORT.md) for detailed examples.

### Go Conventions

- Use **gofmt** and **goimports** for formatting
- Follow [Effective Go](https://go.dev/doc/effective_go) guidelines
- Use **camelCase** for unexported names, **PascalCase** for exported names
- Organize imports: stdlib, external packages, internal packages
- Add godoc comments to all exported types, functions, and constants
- Keep functions small (< 30 lines ideally)
- Use table-driven tests for multiple test cases

### Error Handling

```go
// ❌ Bad
if err != nil {
    return err
}

// ✅ Good
if err != nil {
    return fmt.Errorf("failed to load flag %q: %w", key, err)
}
```

### Naming Conventions

```go
// ❌ Bad
func GetData(id int) (*Data, error)

// ✅ Good
func GetFlagByID(flagID int) (*Flag, error)
```

---

## Testing Guidelines

### Writing Tests

- **Place tests** in `*_test.go` files alongside source code
- **Use table-driven tests** for testing multiple cases
- **Test both happy paths and error cases**
- **Use descriptive test names**: `TestFunctionName_Scenario_ExpectedBehavior`
- **Keep tests independent** - no shared mutable state
- **Use test helpers** for common setup (see `internal/testutil`)

### Example Test Structure

```go
func TestIsRolledOut_VariousPercentages(t *testing.T) {
    tests := []struct {
        name     string
        userID   string
        rollout  int32
        expected bool
    }{
        {
            name:     "0% rollout returns false",
            userID:   "user-123",
            rollout:  0,
            expected: false,
        },
        {
            name:     "100% rollout returns true",
            userID:   "user-123",
            rollout:  100,
            expected: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := IsRolledOut(tt.userID, "test-flag", tt.rollout, "salt")
            if err != nil {
                t.Fatalf("unexpected error: %v", err)
            }
            if result != tt.expected {
                t.Errorf("expected %v, got %v", tt.expected, result)
            }
        })
    }
}
```

### Coverage Expectations

- **Core packages** should have >80% coverage
- **Business logic** (rollout, validation, evaluation) should have >90% coverage
- **Happy paths and error paths** should both be tested
- Run `make test-cover` to generate a coverage report

---

## Pull Request Process

### Before Submitting

1. **Write tests** for your changes
2. **Run all tests** and ensure they pass: `make test-race`
3. **Format your code**: `go fmt ./...`
4. **Update documentation** if you changed behavior or added features
5. **Check for linting issues**: `golangci-lint run` (if installed)
6. **Commit with clear messages** (see below)

### Submitting a Pull Request

1. **Push your branch** to your fork:
   ```bash
   git push origin feature/your-feature-name
   ```

2. **Open a Pull Request** on GitHub against the `main` branch

3. **Fill out the PR template** with:
   - Clear description of the change
   - Link to related issues
   - Screenshots (if applicable)
   - Testing performed

4. **Respond to feedback** from reviewers

5. **Keep your PR updated** with main branch:
   ```bash
   git fetch upstream
   git rebase upstream/main
   git push -f origin feature/your-feature-name
   ```

### PR Review Criteria

- ✅ Tests pass (CI must be green)
- ✅ Code follows style guidelines
- ✅ Changes are well-documented
- ✅ No breaking changes (unless discussed)
- ✅ Commit messages are clear
- ✅ PR description explains the "why" not just the "what"

---

## Commit Message Conventions

### Format

```
<type>: <short summary>

<optional longer description>

<optional footer with issue references>
```

### Types

- **feat:** New feature
- **fix:** Bug fix
- **docs:** Documentation changes
- **style:** Code style/formatting (no functional changes)
- **refactor:** Code refactoring (no functional changes)
- **test:** Adding or updating tests
- **chore:** Maintenance tasks (dependencies, build config)

### Examples

```bash
# Good commit messages
git commit -m "feat: add percentage rollout support for gradual releases"
git commit -m "fix: correct user bucketing edge case when userID is empty"
git commit -m "docs: add troubleshooting section to README"
git commit -m "refactor: extract validation logic into separate functions"

# Bad commit messages (avoid these)
git commit -m "update"
git commit -m "fixes"
git commit -m "wip"
```

---

## Reporting Bugs

### Before Reporting

1. **Search existing issues** to avoid duplicates
2. **Verify the bug** on the latest version
3. **Collect relevant information**: OS, Go version, error messages

### Bug Report Template

When opening a bug report, include:

- **Description**: What happened vs what you expected
- **Steps to Reproduce**: Minimal steps to trigger the issue
- **Environment**: OS, Go version, database version
- **Error Logs**: Relevant error messages or stack traces
- **Screenshots**: If applicable

---

## Suggesting Features

### Feature Request Guidelines

- **Check the roadmap** to see if it's already planned
- **Search existing issues** for similar requests
- **Provide context**: Why is this feature valuable?
- **Describe the use case**: What problem does it solve?
- **Suggest an approach**: How might it work? (optional)

### Feature Scope

This project focuses on:
- ✅ Feature flag management
- ✅ Real-time updates
- ✅ Rollout and targeting
- ✅ API and SDK improvements
- ✅ Developer experience

Out of scope:
- ❌ Full-fledged analytics platform
- ❌ Complex UI frameworks (keep it simple)
- ❌ Non-flag-related features

---

## Questions?

- **Open a discussion** on GitHub for questions
- **Check existing documentation**: README, TESTING.md, AUTH_SETUP.md
- **Review closed issues** for similar topics

Thank you for contributing to goflagship! 🚀
