# GoFlagship

High-performance Feature-Flag & Remote-Config Service in Go.

## Features (MVP)

- REST (Snapshot, SSE), Admin API (CRUD)
- Postgres + sqlc, goose migrations
- Atomic in-memory snapshot + ETag (304)
- Clean Code: lint/test/build, CI
- Observability: /metrics (Prometheus), pprof (später)

## Quickstart

```bash
make db-up
make migrate-up
# (Programmieren startet später)
```
