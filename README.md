# clickUpTask-implementation-pipeline

Go service that turns new ClickUp assignments into ApexSuite-style milestone `.md` plans, persists metadata (Supabase), and emails the result. See `../ClickUpMilestonePlannerMilestone.md` for the full plan.

## Phase 0–7 (current)

- **Phase 0:** Chi router, `GET /v1/health`, repo layout, Docker, CI.
- **Phase 1:** `config.Load()` (godotenv + validation), structured JSON request logs (with request ID), JSON panic recovery, ApexSuite response helpers, `404` / `405` handlers, graceful shutdown.
- **Phase 2:** `DATABASE_URL` (optional) with TLS validation, `db/migrations/001_initial_schema.sql`, `db.Connect` pool, `db.Store` repository (tasks, events, generations), health checks DB when configured.
- **Phase 3:** `POST /v1/webhooks/clickup` — verifies ClickUp `X-Signature` (HMAC-SHA256 hex of raw body), filters assignment-related events, dedupes by `webhook_id:history_item_id` (or body hash), inserts into `clickup_events`.
- **Phase 4:** `services.ClickUpClient` — `GetTask` / `GetTaskComments` against ClickUp API v2 (`CLICKUP_API_TOKEN`, optional `CLICKUP_API_BASE_URL`), 30s HTTP timeout, maps 401/403/404/429 to `ClickUpHTTPError`, normalizes to `models.TaskContext`.
- **Phase 5:** `services.Generator` + `OpenAIGenerator` — embedded prompt (`services/prompts/milestone_prompt.md`), OpenAI Chat Completions (`LLM_API_KEY`, `LLM_MODEL`, optional `LLM_PROVIDER=openai`, optional `LLM_API_BASE_URL`), post-process (CRLF normalize, strip optional ` ```markdown ` fences), section + secret heuristics validation, SHA-256 via `internal/checksum`, filename `{taskId}-{slug}-milestone.md` (`internal/slug`).
- **Phase 6:** `services/storage` — `BlobStore` (`Upload` / `Download` / `SignedDownloadURL`), `SupabaseBlobStore` (Storage REST, `text/markdown`, `x-upsert`), `LocalBlobStore` (under `STORAGE_LOCAL_DIR`), `NewFromConfig`, `PersistMilestone` (upload then `MarkGenerationCompleted`; upload errors call `MarkGenerationFailed`). Bucket defaults to **`milestone-plans`** when `SUPABASE_STORAGE_BUCKET` is unset. Signed URL TTL: `SIGNED_URL_TTL_SECONDS` (default 3600, clamped 60–604800).
- **Phase 7:** `services/email` — `EmailService`, **Resend** (`EMAIL_PROVIDER=resend`, `EMAIL_API_KEY`, `EMAIL_FROM`, `EMAIL_TO`, optional `EMAIL_API_BASE_URL`, optional `EMAIL_MAX_ATTACHMENT_BYTES` default 450000). Small markdown is **attached** as `text/markdown`; larger bodies require **`DownloadURL`** in the payload (link in HTML + text). `SendWithRetry` (3 attempts, capped backoff), `DeliverMilestoneEmail` → `MarkGenerationEmailSent`. Empty / `none` / `noop` provider uses **`NoopEmailService`**. Orchestration calls this in **Phase 8**.

### Supabase Storage bucket (Phase 6)

1. In Supabase: **Storage** → **New bucket** → id **`milestone-plans`** (private is fine; the service uses the **service role** key server-side).
2. Set **`SUPABASE_URL`**, **`SUPABASE_SERVICE_ROLE_KEY`**, and optionally **`SUPABASE_STORAGE_BUCKET`** (defaults to `milestone-plans`).
3. For local dev without Supabase Storage, set **`STORAGE_BACKEND=local`** and **`STORAGE_LOCAL_DIR`** to a writable directory (see `.env.example`). Local backend does not support signed URLs (`ErrSignedURLUnsupported`); use `Download` or a future app route.

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

### Smoke test email (Phase 7 / Resend)

Requires **`EMAIL_PROVIDER=resend`** plus **`EMAIL_API_KEY`**, **`EMAIL_FROM`**, **`EMAIL_TO`** (verified sending domain in Resend).

```bash
go run ./cmd/smoke-email -dry-run
go run ./cmd/smoke-email
go run ./cmd/smoke-email -markdown path/to/small.md
```

`-dry-run` only validates config and payload shape (no API call). A successful run prints to **stderr**; check **EMAIL_TO** inbox and Resend **Logs**.

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
