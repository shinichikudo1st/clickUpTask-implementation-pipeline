# clickUpTask-implementation-pipeline

Go service that turns new ClickUp assignments into ApexSuite-style milestone `.md` plans, persists metadata (Supabase), and emails the result. See `../ClickUpMilestonePlannerMilestone.md` for the full plan.

## Phase 0–4 (current)

- **Phase 0:** Chi router, `GET /v1/health`, repo layout, Docker, CI.
- **Phase 1:** `config.Load()` (godotenv + validation), structured JSON request logs (with request ID), JSON panic recovery, ApexSuite response helpers, `404` / `405` handlers, graceful shutdown.
- **Phase 2:** `DATABASE_URL` (optional) with TLS validation, `db/migrations/001_initial_schema.sql`, `db.Connect` pool, `db.Store` repository (tasks, events, generations), health checks DB when configured.
- **Phase 3:** `POST /v1/webhooks/clickup` — verifies ClickUp `X-Signature` (HMAC-SHA256 hex of raw body), filters assignment-related events, dedupes by `webhook_id:history_item_id` (or body hash), inserts into `clickup_events`.
- **Phase 4:** `services.ClickUpClient` — `GetTask` / `GetTaskComments` against ClickUp API v2 (`CLICKUP_API_TOKEN`, optional `CLICKUP_API_BASE_URL`), 30s HTTP timeout, maps 401/403/404/429 to `ClickUpHTTPError`, normalizes to `models.TaskContext`.

### Supabase migration (Phase 2)

1. In the Supabase project: **SQL Editor** → new query → paste `db/migrations/001_initial_schema.sql` → run.
2. Copy the **database URI** (must include `sslmode=require` or `verify-*`) into `DATABASE_URL` in `.env`.
3. Restart the service; `GET /v1/health` should include `"database":"connected"`.

### ClickUp webhook (Phase 3) — what you need to do

1. **Public HTTPS URL** for your service (ClickUp recommends `https`; local dev use [ngrok](https://ngrok.com/), [Cloudflare Tunnel](https://developers.cloudflare.com/cloudflare-one/connections/connect-apps/), or similar) pointing to `https://<host>/v1/webhooks/clickup`.
2. **Env vars** in `.env` (or deployment secrets):
   - **`DATABASE_URL`** — required for webhooks (handler returns `503` if the DB pool is not configured).
   - **`CLICKUP_WEBHOOK_SECRET`** — the `secret` returned when you **create the webhook** in ClickUp (Settings → Integrations → Webhooks, or [Create Webhook API](https://developer.clickup.com/reference/createwebhook)). Without it, the endpoint returns **`401`**.
   - **`CLICKUP_ASSIGNEE_USER_ID`** (optional) — your ClickUp **user id** as a string (e.g. `184`). When set, only `taskAssigneeUpdated` / `taskUpdated` events where an `assignee_add` history item’s `after.id` matches this user are accepted; `taskCreated` is not filtered by assignee.
3. **Create the webhook** in ClickUp for the Space/List you care about. Subscribe at least to: **`taskCreated`**, **`taskAssigneeUpdated`**, and optionally **`taskUpdated`** (assignee-only updates are detected via `history_items`).
4. Set the webhook **endpoint** to `https://<your-host>/v1/webhooks/clickup` and save. ClickUp will send **`POST`** with **`Content-Type: application/json`** and **`X-Signature`**.
5. **Smoke test:** assign yourself a task (or create one). Check `clickup_events` in Supabase for a new row. Successful responses look like:
   - `{"success":true,"data":{"accepted":true,"event_row_id":"<uuid>","duplicate":false},...}`
   - Replay the same payload: `"duplicate":true` (same `event_id` / dedupe key; no second row).
6. **Reference payload:** `test/fixtures/clickup_webhook_task_created.json` (shape only; re-sign with your secret before sending). Official examples: [ClickUp task webhook payloads](https://developer.clickup.com/docs/webhooktaskpayloads), [Webhook signature](https://developer.clickup.com/docs/webhooksignature).

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
