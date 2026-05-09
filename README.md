# clickUpTask-implementation-pipeline

Go service that turns new ClickUp assignments into ApexSuite-style milestone `.md` plans, persists metadata (Supabase), and emails the result. See `../ClickUpMilestonePlannerMilestone.md` for the full plan.

## Phase 0 (current)

Runnable HTTP skeleton: Chi router, `GET /v1/health`, no external dependencies.

## Requirements

- Go 1.22+

## Run locally

```bash
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
