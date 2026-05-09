-- Phase 2: ClickUp task snapshots, webhook events, milestone generations.
-- Run in Supabase SQL editor or via psql against your project database.

-- gen_random_uuid() is built-in on PostgreSQL 13+ (Supabase).

CREATE TABLE IF NOT EXISTS clickup_tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clickup_task_id TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    description TEXT,
    status TEXT,
    priority TEXT,
    assignee_id TEXT,
    assignee_email TEXT,
    space_id TEXT,
    folder_id TEXT,
    list_id TEXT,
    url TEXT,
    raw_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    last_synced_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS clickup_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id TEXT,
    clickup_task_id TEXT,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL,
    processed BOOLEAN NOT NULL DEFAULT false,
    processed_at TIMESTAMPTZ,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- PostgreSQL UNIQUE allows multiple NULL event_id rows; non-null event_id is deduplicated.
CREATE UNIQUE INDEX IF NOT EXISTS clickup_events_event_id_unique
    ON clickup_events (event_id);

CREATE TABLE IF NOT EXISTS milestone_generations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clickup_task_id TEXT NOT NULL REFERENCES clickup_tasks (clickup_task_id),
    status TEXT NOT NULL CHECK (status IN ('pending', 'processing', 'completed', 'failed')),
    generation_version TEXT NOT NULL,
    prompt_version TEXT NOT NULL,
    model TEXT NOT NULL,
    file_name TEXT,
    storage_bucket TEXT,
    storage_path TEXT,
    markdown_sha256 TEXT,
    email_sent_at TIMESTAMPTZ,
    error_message TEXT,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_clickup_events_task_created
    ON clickup_events (clickup_task_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_milestone_generations_task_created
    ON milestone_generations (clickup_task_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_milestone_generations_status
    ON milestone_generations (status);
