# clickUpTask-implementation-pipeline

Go service that turns new ClickUp assignments into ApexSuite-style milestone `.md` plans, persists metadata (Supabase), and emails the result. See `../ClickUpMilestonePlannerMilestone.md` for the full plan.

## Phase 0–1 (current)

- **Phase 0:** Chi router, `GET /v1/health`, repo layout, Docker, CI.
- **Phase 1:** `config.Load()` (godotenv + validation), structured JSON request logs (with request ID), JSON panic recovery, ApexSuite response helpers, `404` / `405` handlers, graceful shutdown.
- **Phase 2:** `DATABASE_URL` (optional) with TLS validation, `db/migrations/001_initial_schema.sql`, `db.Connect` pool, `db.Store` repository (tasks, events, generations), health checks DB when configured.

### Supabase migration (Phase 2)

1. In the Supabase project: **SQL Editor** → new query → paste `db/migrations/001_initial_schema.sql` → run.
2. Copy the **database URI** (must include `sslmode=require` or `verify-*`) into `DATABASE_URL` in `.env`.
3. Restart the service; `GET /v1/health` should include `"database":"connected"`.

## Requirements

- Go 1.22+
- **API_SECRET** (at least 8 characters) in the environment before `go run .` (see `.env.example`). Optional: put values in `.env` in the repo root; `godotenv` loads it automatically when present.

## Run locally

```bash
set API_SECRET=local-dev-secret-at-least-8-chars
go run .
```

Optional port:

```bash
set PORT=3000
go run .
```

Health check:

```bash
curl -s http://localhost:8080/v1/health
```

## Docker

```bash
docker compose up --build
```

## Module path

`github.com/Apex-Suite-AI/clickup-task-implementation-pipeline`

## Layout

Stub packages match the milestone layout (`handlers`, `config`, `db`, `middleware`, `models`, `services`, `internal`, `templates`, `test`). Phase 1+ fills behavior.
