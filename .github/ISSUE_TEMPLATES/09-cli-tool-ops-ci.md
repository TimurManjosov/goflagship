# Build CLI Tool for Ops and CI Usage

## Problem / Motivation

Currently, managing flags requires:

1. **cURL commands**: Verbose, error-prone, hard to remember
2. **No scripting support**: Difficult to automate flag changes in CI/CD
3. **No bulk operations**: Can't update multiple flags efficiently
4. **No export/import**: Can't backup or migrate flags between environments
5. **No validation**: Easy to make mistakes in JSON payloads

Teams need a **CLI tool** that makes flag management fast, scriptable, and safe.

## Proposed Solution

Build a Go-based CLI tool `flagship` (or `ffs` for "flagship feature service") that:

1. **CRUD Operations**: Create, read, update, delete flags
2. **Bulk Import/Export**: Import flags from YAML/JSON, export to file
3. **Environment Management**: Switch between dev/staging/prod
4. **CI/CD Integration**: Return exit codes, JSON output for scripts
5. **Validation**: Validate flags before uploading
6. **Configuration**: Store API keys and base URLs in config file

## Concrete Tasks

### Phase 1: CLI Framework Setup
- [ ] Choose CLI framework:
  - **cobra** (most popular, used by kubectl, gh)
  - **urfave/cli** (simpler, lighter)
  - **Recommendation**: cobra for rich features
- [ ] Initialize CLI project in `cmd/flagship/`:
  ```
  cmd/flagship/
    main.go
    commands/
      root.go
      create.go
      get.go
      update.go
      delete.go
      list.go
      export.go
      import.go
  ```
- [ ] Set up cobra:
  ```bash
  go get github.com/spf13/cobra@latest
  cobra init --pkg-name github.com/TimurManjosov/goflagship
  ```
- [ ] Create `flagship` command structure

### Phase 2: Configuration Management
- [ ] Support multiple configuration sources (precedence order):
  1. Command flags (`--base-url`, `--api-key`)
  2. Environment variables (`FLAGSHIP_BASE_URL`, `FLAGSHIP_API_KEY`)
  3. Config file (`~/.flagship/config.yaml`)
- [ ] Create config file structure:
  ```yaml
  # ~/.flagship/config.yaml
  default_env: prod
  
  environments:
    dev:
      base_url: http://localhost:8080
      api_key: dev-key-123
    staging:
      base_url: https://staging.example.com
      api_key: staging-key-456
    prod:
      base_url: https://flagship.example.com
      api_key: prod-key-789
  ```
- [ ] Implement `flagship config` commands:
  ```bash
  flagship config init              # Create ~/.flagship/config.yaml
  flagship config set dev.api_key xxx
  flagship config get dev.base_url
  flagship config list
  ```

### Phase 3: Flag CRUD Commands
- [ ] Implement `flagship create`:
  ```bash
  flagship create feature_x \
    --enabled \
    --rollout 50 \
    --config '{"color":"blue"}' \
    --description "New feature X" \
    --env prod
  ```
- [ ] Implement `flagship get`:
  ```bash
  flagship get feature_x --env prod
  # Output: JSON or YAML
  ```
- [ ] Implement `flagship update`:
  ```bash
  flagship update feature_x \
    --enabled=false \
    --env prod
  ```
- [ ] Implement `flagship delete`:
  ```bash
  flagship delete feature_x --env prod
  # Prompt for confirmation unless --force
  ```
- [ ] Implement `flagship list`:
  ```bash
  flagship list --env prod
  flagship list --enabled-only
  flagship list --format json
  ```

### Phase 4: Bulk Operations
- [ ] Implement `flagship export`:
  ```bash
  # Export all flags to YAML
  flagship export --env prod --output flags.yaml
  
  # Export specific flags
  flagship export feature_x banner --output flags.json
  ```
- [ ] Export format (YAML):
  ```yaml
  flags:
    - key: feature_x
      enabled: true
      rollout: 50
      config:
        color: blue
      env: prod
    - key: banner
      enabled: false
      config:
        text: Welcome
      env: prod
  ```
- [ ] Implement `flagship import`:
  ```bash
  # Import flags from file
  flagship import flags.yaml --env prod
  
  # Dry run (validate without applying)
  flagship import flags.yaml --dry-run
  
  # Overwrite existing flags
  flagship import flags.yaml --force
  ```
- [ ] Add validation during import (check schema, required fields)

### Phase 5: Output Formatting
- [ ] Support multiple output formats:
  - `--format json` (for scripts, jq processing)
  - `--format yaml` (human-readable)
  - `--format table` (default, pretty-printed table)
- [ ] Implement table output using `olekukonko/tablewriter`:
  ```
  KEY          ENABLED  ROLLOUT  ENV   UPDATED AT
  feature_x    true     50       prod  2025-01-15 10:30
  banner       false    100      prod  2025-01-14 09:15
  ```
- [ ] Add `--quiet` flag (suppress output, only exit code)
- [ ] Add `--verbose` flag (show request/response details)

### Phase 6: CI/CD Integration
- [ ] Design for scriptability:
  - Exit code 0 on success
  - Exit code 1 on error
  - Exit code 2 on not found
- [ ] Add `--wait` flag for async operations
- [ ] Add `--timeout` flag (default: 30s)
- [ ] Example CI usage:
  ```bash
  # Enable feature in staging after deploy
  flagship update new_checkout --enabled --env staging
  
  # Export prod flags, commit to repo (GitOps)
  flagship export --env prod > prod-flags.yaml
  git add prod-flags.yaml && git commit -m "Update flags"
  
  # Import flags to new environment
  flagship import prod-flags.yaml --env prod-eu
  ```

### Phase 7: Advanced Features
- [ ] Implement `flagship watch`:
  ```bash
  # Watch for flag changes in real-time
  flagship watch --env prod
  # Uses SSE stream, logs changes
  ```
- [ ] Implement `flagship validate`:
  ```bash
  # Validate flag file without importing
  flagship validate flags.yaml
  ```
- [ ] Implement `flagship diff`:
  ```bash
  # Compare flags between environments
  flagship diff --source prod --target staging
  ```
- [ ] Add shell completions (bash, zsh, fish):
  ```bash
  flagship completion bash > /etc/bash_completion.d/flagship
  ```

### Phase 8: Distribution & Documentation
- [ ] Build binaries for multiple platforms:
  ```bash
  # Using goreleaser or manual builds
  GOOS=linux GOARCH=amd64 go build -o bin/flagship-linux-amd64
  GOOS=darwin GOARCH=amd64 go build -o bin/flagship-darwin-amd64
  GOOS=darwin GOARCH=arm64 go build -o bin/flagship-darwin-arm64
  GOOS=windows GOARCH=amd64 go build -o bin/flagship-windows-amd64.exe
  ```
- [ ] Create installation script:
  ```bash
  curl -sSL https://raw.githubusercontent.com/TimurManjosov/go-flagship/main/install.sh | sh
  ```
- [ ] Add to package managers:
  - Homebrew: `brew install go-flagship`
  - Scoop (Windows): `scoop install flagship`
  - apt/yum (Linux): `.deb` and `.rpm` packages
- [ ] Document all commands in README
- [ ] Add man pages (optional)

### Phase 9: Testing
- [ ] Unit tests for each command
- [ ] Integration tests with live server
- [ ] Test error handling (network errors, auth failures)
- [ ] Test config file parsing
- [ ] Test import/export round-trip (export → import → verify same)

## API Changes

No API changes required. CLI uses existing REST API.

## Acceptance Criteria

### Core Commands
- [ ] `flagship create` creates flags successfully
- [ ] `flagship get` retrieves flag details
- [ ] `flagship update` modifies existing flags
- [ ] `flagship delete` removes flags (with confirmation)
- [ ] `flagship list` shows all flags in table format

### Configuration
- [ ] Config file `~/.flagship/config.yaml` is created and used
- [ ] Multiple environments can be configured
- [ ] Flags can override config (`--api-key` takes precedence)
- [ ] Environment variables work (`FLAGSHIP_API_KEY`)

### Bulk Operations
- [ ] `flagship export` exports flags to YAML/JSON
- [ ] `flagship import` imports flags from file
- [ ] Import validates flags before applying
- [ ] `--dry-run` shows what would be imported without applying

### Output & UX
- [ ] Table format is readable and aligned
- [ ] JSON format is valid and parseable
- [ ] YAML format is human-readable
- [ ] Errors are clear and actionable
- [ ] Success messages confirm actions

### CI/CD
- [ ] Exit codes are appropriate (0=success, 1=error, 2=not found)
- [ ] JSON output is script-friendly (`--format json`)
- [ ] Quiet mode suppresses output (`--quiet`)

### Distribution
- [ ] Binaries available for Linux, macOS, Windows
- [ ] Installation script works
- [ ] README documents installation and usage

## Notes / Risks / Edge Cases

### Risks
- **API Key Exposure**: Keys in config file or env vars could leak
  - Mitigation: Warn about permissions (`chmod 600 ~/.flagship/config.yaml`)
  - Consider keychain integration on macOS
- **Network Failures**: CLI must handle timeouts gracefully
  - Mitigation: Add retry logic, clear error messages
- **Breaking API Changes**: CLI depends on backend API stability
  - Mitigation: Version CLI alongside backend, document compatibility

### Edge Cases
- Config file doesn't exist → create with defaults
- No API key configured → prompt user or show helpful error
- Import file with duplicate keys → error or merge?
- Network timeout during bulk import → partial import, rollback?
- Flag already exists during create → error or update?
- Delete non-existent flag → idempotent (success) or error?

### CLI Design Principles
- **Clear error messages**: Never just say "error", explain what and why
- **Confirm destructive actions**: Prompt for `delete` unless `--force`
- **Sensible defaults**: Don't require flags for common cases
- **Composability**: Output JSON for piping to jq, grep, etc.
- **Discoverability**: `flagship --help` and `flagship <cmd> --help` are comprehensive

### Example Usage Scenarios

**DevOps Engineer**
```bash
# Export prod flags for backup
flagship export --env prod > backup.yaml

# Copy prod flags to staging
flagship export --env prod | flagship import --env staging --force
```

**Developer in CI**
```bash
# Enable feature after successful deploy
if [ $? -eq 0 ]; then
  flagship update new_feature --enabled --env staging
fi
```

**QA Tester**
```bash
# Quickly toggle features for testing
flagship update feature_x --enabled
flagship update feature_x --enabled=false
```

### Future Enhancements
- Interactive mode (TUI with bubbletea)
- Flag templates (create from predefined templates)
- Scheduled flag changes (enable at specific time)
- Audit log viewer (`flagship audit`)
- Webhooks test (`flagship webhook test`)
- Integration with vault/secrets management
- Support for TOML/HCL config formats

## Implementation Hints

- CLI will live in `cmd/flagship/` directory
- Use `github.com/spf13/cobra` for command structure
- Use `github.com/spf13/viper` for config management
- Use `github.com/olekukonko/tablewriter` for table output
- Use `gopkg.in/yaml.v3` for YAML parsing
- HTTP client logic can reuse parts of backend (API client)
- Consider creating `internal/client/` package for API wrapper:
  ```go
  type Client struct {
    BaseURL string
    APIKey  string
    HTTPClient *http.Client
  }
  
  func (c *Client) CreateFlag(flag Flag) error
  func (c *Client) GetFlag(key, env string) (*Flag, error)
  // etc.
  ```
- goreleaser config: https://goreleaser.com/

## Labels

`feature`, `cli`, `tooling`, `good-first-issue` (for simple commands), `help-wanted`

## Estimated Effort

**3-4 days**
- Day 1: CLI framework + config + basic CRUD commands
- Day 2: Bulk operations (import/export) + output formatting
- Day 3: Advanced features (watch, diff, validate) + testing
- Day 4: Distribution (binaries, installation) + documentation
