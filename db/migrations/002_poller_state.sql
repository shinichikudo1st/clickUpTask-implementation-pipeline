-- Phase 10: watermark for ClickUp team-task poller (date_updated_gt cursor).

CREATE TABLE IF NOT EXISTS milestone_poller_state (
    id TEXT PRIMARY KEY,
    last_polled_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO milestone_poller_state (id, last_polled_at)
VALUES ('default', timestamptz 'epoch')
ON CONFLICT (id) DO NOTHING;
