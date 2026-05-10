package integration

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/db"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

func requireTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run repository integration tests")
	}
	// Safety guard against accidental non-test targets.
	if !strings.Contains(strings.ToLower(dsn), "test") {
		t.Skip("TEST_DATABASE_URL must point to a test database (URL must include 'test')")
	}

	sqlDB, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	if err := sqlDB.PingContext(ctx); err != nil {
		t.Fatalf("ping db: %v", err)
	}

	if err := runMigrations(ctx, sqlDB); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	if err := resetTables(ctx, sqlDB); err != nil {
		t.Fatalf("reset tables: %v", err)
	}
	t.Cleanup(func() {
		_ = resetTables(context.Background(), sqlDB)
	})

	return sqlDB
}

func runMigrations(ctx context.Context, sqlDB *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS clickup_tasks (
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
		)`,
		`CREATE TABLE IF NOT EXISTS clickup_events (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			event_id TEXT,
			clickup_task_id TEXT,
			event_type TEXT NOT NULL,
			payload JSONB NOT NULL,
			processed BOOLEAN NOT NULL DEFAULT false,
			processed_at TIMESTAMPTZ,
			error_message TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS clickup_events_event_id_unique ON clickup_events (event_id)`,
		`CREATE TABLE IF NOT EXISTS milestone_generations (
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
		)`,
		`CREATE INDEX IF NOT EXISTS idx_clickup_events_task_created ON clickup_events (clickup_task_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_milestone_generations_task_created ON milestone_generations (clickup_task_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_milestone_generations_status ON milestone_generations (status)`,
		`CREATE TABLE IF NOT EXISTS milestone_poller_state (
			id TEXT PRIMARY KEY,
			last_polled_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`INSERT INTO milestone_poller_state (id, last_polled_at) VALUES ('default', timestamptz 'epoch') ON CONFLICT (id) DO NOTHING`,
	}
	for _, stmt := range stmts {
		if _, err := sqlDB.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func resetTables(ctx context.Context, sqlDB *sql.DB) error {
	_, err := sqlDB.ExecContext(ctx, `
TRUNCATE TABLE
	milestone_generations,
	clickup_events,
	clickup_tasks,
	milestone_poller_state
RESTART IDENTITY CASCADE`)
	if err != nil {
		return err
	}
	_, err = sqlDB.ExecContext(ctx, `INSERT INTO milestone_poller_state (id, last_polled_at) VALUES ('default', timestamptz 'epoch')`)
	return err
}

func TestStore_RepositoryRoundTrip(t *testing.T) {
	sqlDB := requireTestDB(t)
	store := db.NewStore(sqlDB)
	ctx := context.Background()

	taskID := "it-task-" + uuid.NewString()
	if err := store.UpsertClickUpTask(ctx, db.ClickUpTaskRow{
		ClickUpTaskID:  taskID,
		Name:           "Integration Task",
		RawPayloadJSON: []byte(`{"task":"integration"}`),
	}); err != nil {
		t.Fatalf("upsert task: %v", err)
	}

	genID, err := store.CreateMilestoneGeneration(ctx, taskID, "pending", "v1", "p1", "gpt-test")
	if err != nil {
		t.Fatalf("create generation: %v", err)
	}
	if err := store.MarkGenerationProcessing(ctx, genID); err != nil {
		t.Fatalf("mark processing: %v", err)
	}
	if err := store.MarkGenerationCompleted(ctx, genID, "file.md", "milestone-plans", "task/file.md", strings.Repeat("a", 64), sql.NullTime{}); err != nil {
		t.Fatalf("mark completed: %v", err)
	}

	latest, err := store.LatestGenerationByTaskID(ctx, taskID)
	if err != nil {
		t.Fatalf("latest generation: %v", err)
	}
	if latest.Status != "completed" {
		t.Fatalf("status=%q", latest.Status)
	}
	if latest.FileName.String != "file.md" {
		t.Fatalf("filename=%q", latest.FileName.String)
	}

	eventID := "evt-" + uuid.NewString()
	firstID, inserted, err := store.InsertClickUpEvent(ctx, db.ClickUpEventRow{
		EventID:       sql.NullString{String: eventID, Valid: true},
		ClickUpTaskID: sql.NullString{String: taskID, Valid: true},
		EventType:     "taskCreated",
		PayloadJSON:   []byte(`{"event":"one"}`),
	})
	if err != nil {
		t.Fatalf("insert event first: %v", err)
	}
	if !inserted {
		t.Fatal("expected first event insert=true")
	}

	secondID, inserted, err := store.InsertClickUpEvent(ctx, db.ClickUpEventRow{
		EventID:       sql.NullString{String: eventID, Valid: true},
		ClickUpTaskID: sql.NullString{String: taskID, Valid: true},
		EventType:     "taskCreated",
		PayloadJSON:   []byte(`{"event":"two"}`),
	})
	if err != nil {
		t.Fatalf("insert event duplicate: %v", err)
	}
	if inserted {
		t.Fatal("expected duplicate event insert=false")
	}
	if secondID != firstID {
		t.Fatalf("duplicate id mismatch: first=%s second=%s", firstID, secondID)
	}

	wm, err := store.GetPollerLastPolledAt(ctx)
	if err != nil {
		t.Fatalf("get poller watermark: %v", err)
	}
	if !wm.Equal(time.Unix(0, 0).UTC()) {
		t.Fatalf("unexpected initial watermark: %s", wm)
	}
	nextWM := time.Now().UTC().Round(time.Second)
	if err := store.SetPollerLastPolledAt(ctx, nextWM); err != nil {
		t.Fatalf("set poller watermark: %v", err)
	}
	gotWM, err := store.GetPollerLastPolledAt(ctx)
	if err != nil {
		t.Fatalf("get poller watermark after set: %v", err)
	}
	if !gotWM.Equal(nextWM) {
		t.Fatalf("watermark mismatch: want=%s got=%s", nextWM, gotWM)
	}
}
