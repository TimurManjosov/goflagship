<h1 align="center">🚀 go-flagship</h1>

<p align="center">
  <b>Open-source feature-flag and configuration service written in Go — with real-time updates via SSE + ETag</b>
</p>

<p align="center">
  <a href="https://github.com/TimurManjosov/go-flagship/stargazers"><img src="https://img.shields.io/github/stars/TimurManjosov/go-flagship?style=flat-square" /></a>
  <a href="https://github.com/TimurManjosov/go-flagship/blob/main/LICENSE"><img src="https://img.shields.io/github/license/TimurManjosov/go-flagship?style=flat-square" /></a>
  <a href="#"><img src="https://img.shields.io/badge/Go-1.25%2B-00ADD8?style=flat-square&logo=go" /></a>
  <a href="#"><img src="https://img.shields.io/badge/TypeScript-SDK-3178C6?style=flat-square&logo=typescript" /></a>
</p>

---

## ✨ Overview

**go-flagship** is a lightweight, high-performance feature-flag system that lets you **toggle features and update configuration live** — without redeploying.

It's like a self-hosted, open-source alternative to **LaunchDarkly**, **GrowthBook**, or **Unleash** — built from scratch with:
- ⚡ **Go** for speed and concurrency
- 🌐 **TypeScript SDK** for live client updates
- 🔄 **ETag + Server-Sent Events** for real-time synchronization
- 📊 **Prometheus telemetry** for observability

---

## 🧩 Features

✅ Real-time flag updates (no refresh required)  
✅ REST API + SSE streaming  
✅ ETag-based caching (304 responses when unchanged)  
✅ In-memory snapshot for fast reads  
✅ Browser SDK with live auto-refresh  
✅ Postgres persistence via Goose migrations  
✅ Prometheus metrics (`/metrics`) + pprof profiling  
✅ Simple demo web client included  
✅ **NEW:** Database-backed API keys with bcrypt hashing  
✅ **NEW:** Role-based access control (readonly, admin, superadmin)  
✅ **NEW:** Audit logging for all admin operations  
✅ **NEW:** API key expiry and revocation support  
✅ **NEW:** Percentage rollouts with deterministic user bucketing  
✅ **NEW:** Multi-variant A/B testing support  
✅ **NEW:** Client-side rollout evaluation in SDK  
✅ **NEW:** CLI tool for managing flags from command line

> Designed to be minimal, composable, and hackable — perfect for learning, startups, or internal infrastructure.

---

## 🖥️ CLI Tool

**flagship** is a command-line tool for managing feature flags, perfect for DevOps, CI/CD pipelines, and automation.

### Installation

Build from source:
```bash
make build-cli
# Binary will be at: bin/flagship
```

Or build manually:
```bash
go build -o flagship ./cmd/flagship
```

### Quick Start

1. **Initialize configuration:**
```bash
flagship config init
```

This creates `~/.flagship/config.yaml` with default environments. Edit it to add your API keys:
```yaml
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

2. **List flags:**
```bash
flagship list --env prod
```

3. **Create a flag:**
```bash
flagship create new_feature --enabled --rollout 50 --env prod
```

### Configuration Priority

Configuration is loaded with the following priority (highest to lowest):
1. **Command flags** (`--base-url`, `--api-key`)
2. **Environment variables** (`FLAGSHIP_BASE_URL`, `FLAGSHIP_API_KEY`)
3. **Config file** (`~/.flagship/config.yaml`)

Example using environment variables:
```bash
export FLAGSHIP_BASE_URL=http://localhost:8080
export FLAGSHIP_API_KEY=my-secret-key
flagship list --env prod
```

### Available Commands

#### Flag Management

**Create a flag:**
```bash
flagship create feature_x --enabled --rollout 50 --env prod
flagship create feature_y --config '{"color":"blue"}' --description "New feature Y"
```

**Get flag details:**
```bash
flagship get feature_x --env prod
flagship get feature_x --env prod --format json
flagship get feature_x --env prod --format yaml
```

**List all flags:**
```bash
flagship list --env prod
flagship list --env prod --enabled-only
flagship list --env prod --format json
```

**Update a flag:**
```bash
flagship update feature_x --enabled=false --env prod
flagship update feature_x --rollout 75 --env prod
flagship update feature_x --config '{"color":"red"}' --env prod
```

**Delete a flag:**
```bash
flagship delete feature_x --env prod
flagship delete feature_x --env prod --force  # Skip confirmation
```

#### Configuration Management

```bash
flagship config init                         # Create config file
flagship config list                         # Show all configuration
flagship config get dev.api_key             # Get specific value
flagship config set dev.api_key my-key      # Set specific value
```

#### Bulk Operations

**Export flags to file:**
```bash
# Export to YAML
flagship export --env prod --output flags.yaml

# Export to stdout
flagship export --env prod > backup.yaml

# Export to JSON
flagship export --env prod --output flags.json --format json
```

**Import flags from file:**
```bash
# Import flags
flagship import flags.yaml --env staging

# Dry run (preview without applying)
flagship import flags.yaml --env staging --dry-run

# Force import (continue on errors)
flagship import flags.yaml --env prod --force
```

### Output Formats

The CLI supports three output formats:

- **`table`** (default): Human-readable table
- **`json`**: JSON for scripting/automation
- **`yaml`**: YAML for human-readable structured data

```bash
flagship list --env prod --format table
flagship list --env prod --format json | jq '.flags[] | select(.enabled == true)'
flagship list --env prod --format yaml
```

### CI/CD Integration

The CLI is designed for automation:

**Exit codes:**
- `0`: Success
- `1`: Error
- `2`: Not found (when applicable)

**Quiet mode:**
```bash
flagship create feature_x --enabled --env prod --quiet
if [ $? -eq 0 ]; then
  echo "Flag created successfully"
fi
```

**Example CI pipeline:**
```bash
#!/bin/bash
# Enable feature after successful deployment

if [ "$DEPLOY_STATUS" = "success" ]; then
  flagship update new_checkout --enabled --env staging --quiet
  echo "Feature flag enabled in staging"
fi
```

**GitOps workflow:**
```bash
# Export prod flags and commit to git
flagship export --env prod > prod-flags.yaml
git add prod-flags.yaml
git commit -m "Update production flags"
git push

# Import flags to new environment
flagship import prod-flags.yaml --env prod-eu
```

### Advanced Examples

**Copy flags between environments:**
```bash
flagship export --env prod | flagship import --env staging --force
```

**Filter and manipulate with jq:**
```bash
# Get all enabled flags
flagship list --env prod --format json | jq '.flags[] | select(.enabled == true) | .key'

# Get flags with rollout < 100%
flagship list --env prod --format json | jq '.flags[] | select(.rollout < 100)'
```

**Batch operations:**
```bash
# Export all environments
for env in dev staging prod; do
  flagship export --env $env --output "backup-$env.yaml"
done

# Disable all flags (for maintenance)
flagship list --env prod --format json | jq -r '.flags[].key' | while read key; do
  flagship update "$key" --enabled=false --env prod
done
```

---

## 🏗️ Architecture

```
┌──────────────┐ ┌──────────────┐
│  Database    │◄────►│  Go Backend  │───┐
│ (PostgreSQL) │      │  REST + SSE  │   │
└──────────────┘      └──────────────┘   │
                                          ▼
                                    ┌──────────────┐
                                    │  TypeScript  │
                                    │  Browser SDK │
                                    └──────────────┘
                                          │
                                          ▼
                                    ┌──────────────┐
                                    │   Web App    │
                                    │ (index.html) │
                                    └──────────────┘
```

---

## ⚙️ Installation

### 1. Clone and install dependencies
```bash
git clone https://github.com/TimurManjosov/go-flagship.git
cd go-flagship
go mod tidy
```

### 2. Configure your environment
Create a `.env` file or export variables:

```bash
DB_DSN=postgres://user:pass@localhost:5432/flagship?sslmode=disable
HTTP_ADDR=:8080
METRICS_ADDR=:9090
ADMIN_API_KEY=admin-123
ENV=prod
```

### 3. Run database migrations
```bash
go install github.com/pressly/goose/v3/cmd/goose@latest
goose -dir internal/db/migrations postgres "$DB_DSN" up
```

### 4. Run the server
```bash
go run ./cmd/server
```

---

## 🧠 API Endpoints

### Flag Management

| Method | Endpoint              | Description                                                           |
|--------|-----------------------|-----------------------------------------------------------------------|
| GET    | `/healthz`            | Health check                                                          |
| GET    | `/v1/flags/snapshot`  | Fetch all flags + ETag                                                |
| GET    | `/v1/flags/stream`    | Subscribe via SSE for updates                                         |
| POST   | `/v1/flags`           | Create/update flag (requires admin role)                              |
| DELETE | `/v1/flags`           | Delete flag by key & env (requires admin role)                        |

### Authentication & Security (NEW)

| Method | Endpoint                  | Description                                  |
|--------|---------------------------|----------------------------------------------|
| POST   | `/v1/admin/keys`          | Create API key (requires superadmin role)    |
| GET    | `/v1/admin/keys`          | List all API keys (requires admin role)      |
| DELETE | `/v1/admin/keys/:id`      | Revoke API key (requires superadmin role)    |
| GET    | `/v1/admin/audit-logs`    | View audit logs (requires admin role)        |

📚 **See [AUTH_SETUP.md](AUTH_SETUP.md) for detailed authentication setup and usage guide.**

### Example flag creation
```bash
curl -X POST http://localhost:8080/v1/flags \
  -H "Authorization: Bearer admin-123" \
  -H "Content-Type: application/json" \
  -d '{"key":"banner_message","enabled":true,"env":"prod","config":{"text":"Hello world"}}'
```

### Example flag deletion
```bash
curl -X DELETE "http://localhost:8080/v1/flags?key=banner_message&env=prod" \
  -H "Authorization: Bearer admin-123"
```

---

## 💻 TypeScript SDK

### Installation
Clone or import from local path (will be published to npm soon):

```bash
cd sdk
npm install
```

### Usage (browser)
```html
<script type="module">
  import { FlagshipClient } from './dist/flagshipClient.js';

  const client = new FlagshipClient({ baseUrl: 'http://localhost:8080' });
  await client.init();

  // Get flags
  console.log(client.keys());
  console.log(client.isEnabled('banner_message'));

  // Auto-update UI on live change
  client.on('update', () => {
    const cfg = client.getConfig('banner_message');
    document.getElementById('banner').textContent = cfg?.text ?? '';
  });
</script>
```

### API (SDK)

| Method              | Description                                           |
|---------------------|-------------------------------------------------------|
| `init()`            | Fetch snapshot + connect SSE                          |
| `on(event, fn)`     | Listen to 'ready', 'update', 'error'                  |
| `isEnabled(key)`    | Returns boolean (considers rollout percentage)        |
| `getConfig(key)`    | Returns config object                                 |
| `getVariant(key)`   | Returns assigned variant name for A/B tests           |
| `getVariantConfig(key)` | Returns config for the assigned variant           |
| `setUser(user)`     | Set user context for rollout evaluation               |
| `getUser()`         | Get current user context                              |
| `keys()`            | Returns all flag keys                                 |
| `close()`           | Stops the stream                                      |

---

## 🎯 Percentage Rollouts & A/B Testing

go-flagship supports **gradual rollouts** and **A/B testing** with deterministic user bucketing.

### Configuration

Set a stable rollout salt in your environment (important for production!):

```bash
# In .env or environment variables
ROLLOUT_SALT=your-stable-production-salt-v1
```

⚠️ **Warning:** Changing the salt will redistribute all users to different buckets!

### Basic Percentage Rollout

Create a flag with a rollout percentage:

```bash
curl -X POST http://localhost:8080/v1/flags \
  -H "Authorization: Bearer admin-123" \
  -H "Content-Type: application/json" \
  -d '{
    "key": "new_checkout_flow",
    "enabled": true,
    "rollout": 25,
    "env": "prod",
    "config": {"version": "v2"}
  }'
```

This enables the flag for approximately 25% of users.

### Multi-Variant A/B Testing

Create a flag with multiple variants:

```bash
curl -X POST http://localhost:8080/v1/flags \
  -H "Authorization: Bearer admin-123" \
  -H "Content-Type: application/json" \
  -d '{
    "key": "checkout_experiment",
    "enabled": true,
    "rollout": 100,
    "variants": [
      {"name": "control", "weight": 50, "config": {"layout": "standard"}},
      {"name": "express", "weight": 30, "config": {"layout": "single-page"}},
      {"name": "premium", "weight": 20, "config": {"layout": "vip"}}
    ],
    "env": "prod"
  }'
```

### SDK Usage with Rollouts

```typescript
import { FlagshipClient } from './dist/flagshipClient.js';

// Create client with user context
const client = new FlagshipClient({
  baseUrl: 'http://localhost:8080',
  user: { id: 'user-123' }  // Required for rollouts
});

await client.init();

// Check if user is in rollout
if (client.isEnabled('new_checkout_flow')) {
  // User is in the 25% rollout
  showNewCheckout();
}

// Get assigned variant for A/B test
const variant = client.getVariant('checkout_experiment');
if (variant === 'express') {
  showExpressCheckout();
} else if (variant === 'premium') {
  showPremiumCheckout();
} else {
  showStandardCheckout();
}

// Get variant-specific config
const config = client.getVariantConfig('checkout_experiment');
console.log(config?.layout); // 'standard', 'single-page', or 'vip'

// Update user context (e.g., after login)
client.setUser({ id: 'logged-in-user-456' });
```

### How It Works

1. **Deterministic Hashing**: Uses xxHash (server) / MurmurHash (client) to hash `userID + flagKey + salt`
2. **Bucket Assignment**: Hash is mapped to a bucket (0-99)
3. **Rollout Check**: If bucket < rollout percentage, user sees the feature
4. **Variant Assignment**: For A/B tests, bucket determines which variant based on cumulative weights

### Key Properties

- **Deterministic**: Same user always gets the same result for a flag
- **Even Distribution**: Users are evenly distributed across buckets
- **Consistent**: Same assignment across client and server (when using same salt)
- **Safe**: Changing rollout from 10% to 20% only adds users, never removes

---

## 📊 Metrics

| Endpoint             | Description                    |
|----------------------|--------------------------------|
| `:9090/metrics`      | Prometheus metrics             |
| `:9090/debug/pprof`  | Go profiling tools             |

Example metrics include:
- `http_requests_total`
- `snapshot_flags`
- `sse_clients`
- `go_memstats_*`

---

## 🧱 Folder Structure

```
go-flagship/
├── cmd/
│   └── server/          # Entry point
├── internal/
│   ├── api/             # HTTP handlers
│   ├── db/              # Goose migrations
│   ├── repo/            # DB repository layer
│   ├── snapshot/        # In-memory flag cache
│   ├── telemetry/       # Prometheus & pprof
│   └── config/          # Environment config
├── sdk/                 # TypeScript SDK
│   └── index.html       # Minimal live demo
└── README.md
```

---

## 🚀 Running the Demo

1. Start your Go API (`:8080`)

2. Serve the SDK demo locally:
```bash
cd sdk
npx http-server -p 3000 -c-1
```

3. Open `http://localhost:3000/index.html`

4. Use cURL to add or update flags → watch the page auto-update instantly

---

## 🧪 Testing

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests with race detector (recommended)
go test -race ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# Using Makefile
make test           # Run all tests
make test-verbose   # Run with verbose output
make test-race      # Run with race detector
make test-cover     # Generate coverage report
```

### Test Coverage

- **Snapshot Package**: Unit tests for flag snapshots, ETag generation, atomic updates
- **API Package**: Integration tests for REST endpoints, authentication, validation
- **SSE Tests**: Server-Sent Events connection, event delivery, multiple clients
- **Concurrency Tests**: Race condition tests with 50-100 concurrent goroutines
- **Rollout Package**: Unit tests for hashing, bucket distribution, variant selection
- **SDK Rollout Tests**: Client-side rollout evaluation tests
- **Total**: 70+ automated tests covering critical paths

---

## 📅 Roadmap

- [ ] Node.js SDK support
- [ ] React admin dashboard
- [ ] Flag targeting (country, plan, userId)
- [x] Percentage rollouts
- [x] Multi-variant A/B testing
- [x] JWT or API-key authentication
- [x] CLI tool for ops and CI/CD
- [ ] Docker Compose setup
- [x] Unit + integration tests
- [ ] Publish SDK on npm

---

## ❤️ Contributing

Pull requests and discussions are welcome!

1. Fork the repository
2. Create your feature branch
3. Commit changes with a clear message
4. Push to your branch
5. Create a PR 🎉

---

## 🪶 License

MIT License © 2025 Timur Manjosov

---

<p align="center">
  <sub>Built with Go, TypeScript, and curiosity — by a developer who believes in precision, simplicity, and flow.</sub>
</p>
