# clickUpTask-implementation-pipeline

Go service that turns new ClickUp assignments into ApexSuite-style milestone `.md` plans, persists metadata (Supabase), and emails the result. See `../ClickUpMilestonePlannerMilestone.md` for the full plan.

## Phase 0–11 (current)

- **Phase 0:** Chi router, `GET /v1/health`, repo layout, Docker, CI.
- **Phase 1:** `config.Load()` (godotenv + validation), structured JSON request logs (with request ID), JSON panic recovery, ApexSuite response helpers, `404` / `405` handlers, graceful shutdown.
- **Phase 2:** `DATABASE_URL` (optional) with TLS validation, `db/migrations/001_initial_schema.sql` plus **`002_poller_state.sql`** (poller watermark), `db.Connect` pool, `db.Store` repository (tasks, events, generations, poller state), health checks DB when configured.
- **Phase 3:** `POST /v1/webhooks/clickup` — verifies ClickUp `X-Signature` (HMAC-SHA256 hex of raw body), filters assignment-related events, dedupes by `webhook_id:history_item_id` (or body hash), inserts into `clickup_events`.
- **Phase 4:** `services.ClickUpClient` — `GetTask` / `GetTaskComments` against ClickUp API v2 (`CLICKUP_API_TOKEN`, optional `CLICKUP_API_BASE_URL`), 30s HTTP timeout, maps 401/403/404/429 to `ClickUpHTTPError`, normalizes to `models.TaskContext`.
- **Phase 5:** `services.Generator` + `OpenAIGenerator` — embedded prompt (`services/prompts/milestone_prompt.md`) instructs ApexSuite-style milestones (metadata ribbon, `---` separators, decision tables, fenced text architecture diagrams, optional API/data contract, SQL index hints, directory tree, ASCII-only `### Phase N - Title` phase headings), OpenAI Chat Completions (`LLM_API_KEY`, `LLM_MODEL`, optional `LLM_PROVIDER=openai`, optional `LLM_API_BASE_URL`), post-process (CRLF normalize, strip optional markdown code fences), section + secret heuristics validation, SHA-256 via `internal/checksum`, filename `{taskId}-{slug}-milestone.md` (`internal/slug`).
- **Phase 6:** `services/storage` — `BlobStore` (`Upload` / `Download` / `SignedDownloadURL`), `SupabaseBlobStore` (Storage REST, `text/markdown`, `x-upsert`), `LocalBlobStore` (under `STORAGE_LOCAL_DIR`), `NewFromConfig`, `PersistMilestone` (upload then `MarkGenerationCompleted`; upload errors call `MarkGenerationFailed`). Bucket defaults to **`milestone-plans`** when `SUPABASE_STORAGE_BUCKET` is unset. Signed URL TTL: `SIGNED_URL_TTL_SECONDS` (default **900** when unset or `0`, clamped 60–604800).
- **Phase 7:** `services/email` — `EmailService`, **Resend** (`EMAIL_PROVIDER=resend`, `EMAIL_API_KEY`, `EMAIL_FROM`, `EMAIL_TO`, optional `EMAIL_API_BASE_URL`, optional `EMAIL_MAX_ATTACHMENT_BYTES` default 450000). Small markdown is **attached** as `text/markdown`; larger bodies require **`DownloadURL`** in the payload (link in HTML + text). `SendWithRetry` (3 attempts, capped backoff), `DeliverMilestoneEmail` → `MarkGenerationEmailSent`. Empty / `none` / `noop` provider uses **`NoopEmailService`**. Orchestration calls this in **Phase 8**.
- **Phase 8:** `services.TryNewPlanner` + `Planner.GenerateForTask` — after a **new** `clickup_events` insert (assignment-related webhook, not a dedupe replay), runs **async** ClickUp fetch → upsert `clickup_tasks` → `pending` → `processing` → LLM → `storage.PersistMilestone` → signed URL (when supported) → `email.DeliverMilestoneEmail`. Skips work if the task already has a **`completed`** generation and `force` is false. Webhook marks `clickup_events.processed` / `error_message` after the planner finishes.
- **Phase 9:** `POST /v1/tasks/{clickup_task_id}/generate` and `GET /v1/tasks/{clickup_task_id}/plan` — require **`Authorization: Bearer <API_SECRET>`** or **`X-API-Secret`**. Generate returns **`202`** and runs the same pipeline **asynchronously** (`?force=true` or JSON `{"force":true}`). Plan returns the latest `milestone_generations` row plus **`download_url`** when status is **`completed`** and signed URLs are supported.
- **Phase 10:** `services.RunPollCycle` + `ClickUpClient.ListTeamTasksForAssignee` — optional backfill using **`GET /team/{team_id}/task`** with **`assignees[]`** + **`date_updated_gt`** (watermark in **`milestone_poller_state`**, 2-minute overlap). Skips tasks with a **completed** latest generation (same as webhooks). **`go run ./cmd/poll-milestones`** for cron/one-shot; or set **`CLICKUP_POLLER_ENABLED=true`** and **`CLICKUP_POLL_INTERVAL_SECONDS` ≥ 30** for an in-process ticker in **`go run .`**. Requires **`CLICKUP_TEAM_ID`** and **`CLICKUP_ASSIGNEE_USER_ID`**. Default **`CLICKUP_POLLER_LOOKBACK_HOURS=168`** applies when the watermark is still at epoch (first run).
- **Phase 11:** **`internal/safelog.Redact`** on panic stacks, OpenAI error snippets, and `log` paths that include `err.Error()`; **`ValidateGeneratedMarkdown`** rejects additional secret-shaped output (PEM private key blocks, AWS `AKIA…` access key ids, plus existing OpenAI/Stripe/Bearer/JWT heuristics). **No full LLM prompts** are persisted in Postgres (only `prompt_version` / generation metadata). Keep the Storage bucket **private**; clients use **short-lived signed URLs** from the service role. Secrets stay in env / deployment config only.

### Supabase Storage bucket (Phase 6)

1. In Supabase: **Storage** → **New bucket** → id **`milestone-plans`**. Prefer **private** (no public read): the service uses the **service role** key server-side and returns **signed download URLs** with TTL from **`SIGNED_URL_TTL_SECONDS`** (default 15 minutes when unset).
2. Set **`SUPABASE_URL`**, **`SUPABASE_SERVICE_ROLE_KEY`**, and optionally **`SUPABASE_STORAGE_BUCKET`** (defaults to `milestone-plans`).
3. For local dev without Supabase Storage, set **`STORAGE_BACKEND=local`** and **`STORAGE_LOCAL_DIR`** to a writable directory (see `.env.example`). Local backend does not support signed URLs (`ErrSignedURLUnsupported`); use `Download` or a future app route.

### Supabase migration (Phase 2)

1. In the Supabase project: **SQL Editor** → new query → paste `db/migrations/001_initial_schema.sql` → run.
2. For the poller (Phase 10): run **`db/migrations/002_poller_state.sql`** in the same SQL editor (or append to your migration runner).
3. Copy the **database URI** (must include `sslmode=require` or `verify-*`) into `DATABASE_URL` in `.env`.
4. Restart the service; `GET /v1/health` should include `"database":"connected"`.

### ClickUp webhook (Phase 3) — what you need to do

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

### Smoke test email (Phase 7 / Resend)

Requires **`EMAIL_PROVIDER=resend`** plus **`EMAIL_API_KEY`**, **`EMAIL_FROM`**, **`EMAIL_TO`** (verified sending domain in Resend).

```bash
go run ./cmd/smoke-email -dry-run
go run ./cmd/smoke-email
go run ./cmd/smoke-email -markdown path/to/small.md
```

`-dry-run` only validates config and payload shape (no API call). A successful run prints to **stderr**; check **EMAIL_TO** inbox and Resend **Logs**.

### Poller one-shot (Phase 10)

Requires the same **`DATABASE_URL`**, planner, and ClickUp env as the main service, plus **`CLICKUP_TEAM_ID`** and **`CLICKUP_ASSIGNEE_USER_ID`**. Run **`db/migrations/002_poller_state.sql`** once. Then:

```bash
set CLICKUP_POLLER_ENABLED=true
set CLICKUP_TEAM_ID=your_team_id
set CLICKUP_ASSIGNEE_USER_ID=your_user_id
go run ./cmd/poll-milestones
```

For a repeating in-process poll, set **`CLICKUP_POLL_INTERVAL_SECONDS`** to **30** or higher and start **`go run .`** (ticker runs only when the milestone planner is enabled).

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
