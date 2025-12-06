.PHONY: test test-race test-cover test-verbose clean build run build-cli

# Run all tests
test:
	go test ./...

# Run tests with race detector
test-race:
	go test -race ./...

# Run tests with coverage
test-cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run tests with verbose output
test-verbose:
	go test -v ./...

# Clean build artifacts and test cache
clean:
	go clean -testcache
	rm -f coverage.out coverage.html
	rm -f bin/server bin/flagship

# Build the server
build:
	go build -o bin/server ./cmd/server

# Build the CLI tool
build-cli:
	go build -o bin/flagship ./cmd/flagship

# Build everything
build-all: build build-cli

# Run the server
run:
	go run ./cmd/server
