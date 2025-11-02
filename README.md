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

> Designed to be minimal, composable, and hackable — perfect for learning, startups, or internal infrastructure.

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
                                    │ (demo.html)  │
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

| Method | Endpoint              | Description                                                           |
|--------|-----------------------|-----------------------------------------------------------------------|
| GET    | `/healthz`            | Health check                                                          |
| GET    | `/v1/flags/snapshot`  | Fetch all flags + ETag                                                |
| GET    | `/v1/flags/stream`    | Subscribe via SSE for updates                                         |
| POST   | `/v1/flags`           | Create/update flag (requires `Authorization: Bearer admin-123`)       |

### Example flag creation
```bash
curl -X POST http://localhost:8080/v1/flags \
  -H "Authorization: Bearer admin-123" \
  -H "Content-Type: application/json" \
  -d '{"key":"banner_message","enabled":true,"env":"prod","config":{"text":"Hello world"}}'
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

| Method          | Description                                 |
|-----------------|---------------------------------------------|
| `init()`        | Fetch snapshot + connect SSE                |
| `on(event, fn)` | Listen to 'ready', 'update', 'error'        |
| `isEnabled(key)`| Returns boolean                             |
| `getConfig(key)`| Returns config object                       |
| `keys()`        | Returns all flag keys                       |
| `close()`       | Stops the stream                            |

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
│   └── demo.html        # Minimal live demo
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

3. Open `http://localhost:3000/demo.html`

4. Use cURL to add or update flags → watch the page auto-update instantly

---

## 📅 Roadmap

- [ ] Node.js SDK support
- [ ] React admin dashboard
- [ ] Flag targeting (country, plan, userId)
- [ ] Percentage rollouts
- [ ] JWT or API-key authentication
- [ ] Docker Compose setup
- [ ] Unit + integration tests
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
