# clickUpTask-implementation-pipeline

Go service that turns new ClickUp assignments into ApexSuite-style milestone `.md` plans, persists metadata (Supabase), and emails the result.

## Current Capabilities

- HTTP service with Chi router, health endpoint, JSON responses, request logging, panic recovery, graceful shutdown, and Docker/CI support.
- Config + validation for core app settings, ClickUp, LLM, storage, email, and poller behavior.
- PostgreSQL repository layer (`db.Store`) with task snapshots, webhook events, generation metadata, and poller watermark persistence.
- ClickUp ingestion via signed webhooks and optional poller backfill (`GET /team/{team_id}/task` with assignee/date filters).
- Milestone generation via OpenAI Chat Completions with markdown validation and secret-like output scanning.
- Storage backends for Supabase and local filesystem with short-lived signed URL support.
- Email delivery via Resend (attachment or download link) with retry/backoff.
- Manual authenticated task APIs for triggering generation and reading the latest plan state.
- Security hardening: server-side secrets from env, log redaction, private-by-default storage guidance, and no full prompt persistence.

### Supabase Storage bucket

1. In Supabase: **Storage** → **New bucket** → id **`milestone-plans`**. Prefer **private** (no public read): the service uses the **service role** key server-side and returns **signed download URLs** with TTL from **`SIGNED_URL_TTL_SECONDS`** (default 15 minutes when unset).
2. Set **`SUPABASE_URL`**, **`SUPABASE_SERVICE_ROLE_KEY`**, and optionally **`SUPABASE_STORAGE_BUCKET`** (defaults to `milestone-plans`).
3. For local dev without Supabase Storage, set **`STORAGE_BACKEND=local`** and **`STORAGE_LOCAL_DIR`** to a writable directory (see `.env.example`). Local backend does not support signed URLs (`ErrSignedURLUnsupported`); use `Download` or a future app route.

### Supabase migration

1. In the Supabase project: **SQL Editor** → new query → paste `db/migrations/001_initial_schema.sql` → run.
2. For the poller watermark table: run **`db/migrations/002_poller_state.sql`** in the same SQL editor (or append to your migration runner).
3. Copy the **database URI** (must include `sslmode=require` or `verify-*`) into `DATABASE_URL` in `.env`.
4. Restart the service; `GET /v1/health` should include `"database":"connected"`.

### ClickUp webhook setup

1. **Public HTTPS URL** for your service (ClickUp recommends `https`; local dev use [ngrok](https://ngrok.com/), [Cloudflare Tunnel](https://developers.cloudflare.com/cloudflare-one/connections/connect-apps/), or similar) pointing to `https://<host>/v1/webhooks/clickup`.
2. **Env vars** in `.env` (or deployment secrets):
   - **`DATABASE_URL`** — required for webhooks (handler returns `503` if the DB pool is not configured).
   - **`CLICKUP_WEBHOOK_SECRET`** — the `secret` returned when you **create the webhook** in ClickUp (Settings → Integrations → Webhooks, or [Create Webhook API](https://developer.clickup.com/reference/createwebhook)). Without it, the endpoint returns **`401`**.
   - **`CLICKUP_ASSIGNEE_USER_ID`** (optional) — your ClickUp **user id** as a string (e.g. `184`). When set, **`taskCreated` is ignored** for milestone generation (tasks created unassigned or assigned to someone else won’t run). Only **`taskAssigneeUpdated`** and **`taskUpdated`** with an `assignee_add` whose `after.id` matches this user are accepted. When unset, **`taskCreated`** is still accepted like before.
3. **Create the webhook** in ClickUp for the Space/List you care about. Subscribe at least to: **`taskCreated`**, **`taskAssigneeUpdated`**, and optionally **`taskUpdated`** (assignee-only updates are detected via `history_items`).
4. Set the webhook **endpoint** to `https://<your-host>/v1/webhooks/clickup` and save. ClickUp will send **`POST`** with **`Content-Type: application/json`** and **`X-Signature`**.
5. **Smoke test:** assign yourself a task (or create one). Check `clickup_events` in Supabase for a new row. Successful responses look like:
   - `{"success":true,"data":{"accepted":true,"event_row_id":"<uuid>","duplicate":false},...}`
   - Replay the same payload: `"duplicate":true` (same `event_id` / dedupe key; no second row).
6. **Reference payload:** `test/fixtures/clickup_webhook_task_created.json` (shape only; re-sign with your secret before sending). Official examples: [ClickUp task webhook payloads](https://developer.clickup.com/docs/webhooktaskpayloads), [Webhook signature](https://developer.clickup.com/docs/webhooksignature).

## Requirements

- Go 1.22+
- **API_SECRET** (at least 8 characters) in the environment before `go run .` (see `.env.example`). Optional: put values in `.env` in the repo root; `godotenv` loads it automatically when present.

## Manual LLM smoke test (real OpenAI call)

Uses `LLM_API_KEY` (and optional `LLM_MODEL`, `LLM_API_BASE_URL`) from `.env` or the environment. This spends a small amount of API credit and verifies prompts, truncation, and markdown validation against the live model.

**Option A — JSON task context** (copy and edit `test/fixtures/smoke_task_context.example.json`):

```bash
go run ./cmd/smoke-llm -context path/to/your-task.json
go run ./cmd/smoke-llm -context path/to/your-task.json -out milestone.md
```

**Option B — fetch a real ClickUp task** (`CLICKUP_API_TOKEN` in `.env`):

```bash
go run ./cmd/smoke-llm -clickup-task YOUR_TASK_ID
go run ./cmd/smoke-llm -clickup-task YOUR_TASK_ID -comments
go run ./cmd/smoke-llm -clickup-task YOUR_TASK_ID -dump-context > task-context.json
```

Metadata (model, filename, sha256) is printed to **stderr**; the markdown body goes to **stdout** unless `-out` is set.

### Smoke test email (Resend)

Requires **`EMAIL_PROVIDER=resend`** plus **`EMAIL_API_KEY`**, **`EMAIL_FROM`**, **`EMAIL_TO`** (verified sending domain in Resend).

```bash
go run ./cmd/smoke-email -dry-run
go run ./cmd/smoke-email
go run ./cmd/smoke-email -markdown path/to/small.md
```

`-dry-run` only validates config and payload shape (no API call). A successful run prints to **stderr**; check **EMAIL_TO** inbox and Resend **Logs**.

### Poller one-shot

Requires the same **`DATABASE_URL`**, planner, and ClickUp env as the main service, plus **`CLICKUP_TEAM_ID`** and **`CLICKUP_ASSIGNEE_USER_ID`**. Run **`db/migrations/002_poller_state.sql`** once. Then:

```bash
set CLICKUP_POLLER_ENABLED=true
set CLICKUP_TEAM_ID=your_team_id
set CLICKUP_ASSIGNEE_USER_ID=your_user_id
go run ./cmd/poll-milestones
```

For a repeating in-process poll, set **`CLICKUP_POLL_INTERVAL_SECONDS`** to **30** or higher and start **`go run .`** (ticker runs only when the milestone planner is enabled).

### Repository integration test

Integration tests in `test/integration` use a real Postgres database through `TEST_DATABASE_URL`:

```bash
set TEST_DATABASE_URL=postgres://user:pass@localhost:5432/apexsuite_test?sslmode=disable
go test ./test/integration -count=1
```

Notes:

- Tests auto-create required tables/indexes if missing.
- Tests truncate the project tables before/after each run.
- For safety, tests skip unless `TEST_DATABASE_URL` contains `test`.

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

Packages match the milestone layout (`handlers`, `config`, `db`, `middleware`, `models`, `services`, `internal`, `templates`, `test`). The canonical LLM prompt is `services/prompts/milestone_prompt.md` (embedded at build time); `templates/milestone_prompt.md` points to it.

## Architecture (V1)

The pipeline is deterministic and async:

1. ClickUp webhook or poller identifies a task.
2. Service fetches normalized task context from ClickUp.
3. Planner creates a generation row and calls the LLM generator.
4. Markdown is validated and uploaded to storage.
5. Generation metadata is updated and email is delivered (attachment or link).

Core components:

- `handlers` for webhook/manual HTTP entry points.
- `services` for ClickUp API, generation, orchestration, storage, email, and poller.
- `db.Store` as repository for tasks/events/generations/poller state.
- `middleware` for auth, request logging, and panic recovery.

## HTTP Endpoints

- `GET /v1/health`: service health (and DB state when configured).
- `POST /v1/webhooks/clickup`: ClickUp webhook intake (signature validated).
- `POST /v1/tasks/{clickup_task_id}/generate`: authenticated manual async generation (`?force=true` optional).
- `GET /v1/tasks/{clickup_task_id}/plan`: authenticated latest generation status + signed URL when available.

## Environment Variables

Use `.env.example` as the source of truth. Typical minimum set:

- Core: `API_SECRET`, `PORT` (optional), `DATABASE_URL`.
- ClickUp: `CLICKUP_API_TOKEN`, `CLICKUP_WEBHOOK_SECRET`, `CLICKUP_TEAM_ID`, `CLICKUP_ASSIGNEE_USER_ID`.
- LLM: `LLM_API_KEY`, `LLM_MODEL` (optional), `LLM_API_BASE_URL` (optional).
- Storage: `STORAGE_BACKEND`, `SUPABASE_URL`, `SUPABASE_SERVICE_ROLE_KEY`, `SUPABASE_STORAGE_BUCKET`, `SIGNED_URL_TTL_SECONDS`.
- Email: `EMAIL_PROVIDER`, `EMAIL_API_KEY`, `EMAIL_FROM`, `EMAIL_TO`.
