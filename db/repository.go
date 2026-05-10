package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Store performs parameterized SQL for ClickUp and milestone metadata.
type Store struct {
	db *sql.DB
}

// NewStore returns a Store backed by db. db must not be nil.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// UpsertClickUpTask inserts or updates a task snapshot by clickup_task_id.
func (s *Store) UpsertClickUpTask(ctx context.Context, row ClickUpTaskRow) error {
	if row.ClickUpTaskID == "" {
		return errors.New("clickup_task_id is required")
	}
	if row.Name == "" {
		return errors.New("name is required")
	}
	if len(row.RawPayloadJSON) == 0 {
		row.RawPayloadJSON = []byte("{}")
	}
	if row.LastSyncedAt.IsZero() {
		row.LastSyncedAt = time.Now().UTC()
	}

	const q = `
INSERT INTO clickup_tasks (
    clickup_task_id, name, description, status, priority,
    assignee_id, assignee_email, space_id, folder_id, list_id, url, raw_payload, last_synced_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12::jsonb, $13
)
ON CONFLICT (clickup_task_id) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    status = EXCLUDED.status,
    priority = EXCLUDED.priority,
    assignee_id = EXCLUDED.assignee_id,
    assignee_email = EXCLUDED.assignee_email,
    space_id = EXCLUDED.space_id,
    folder_id = EXCLUDED.folder_id,
    list_id = EXCLUDED.list_id,
    url = EXCLUDED.url,
    raw_payload = EXCLUDED.raw_payload,
    last_synced_at = EXCLUDED.last_synced_at,
    updated_at = now()
`
	_, err := s.db.ExecContext(ctx, q,
		row.ClickUpTaskID,
		row.Name,
		row.Description,
		row.Status,
		row.Priority,
		row.AssigneeID,
		row.AssigneeEmail,
		row.SpaceID,
		row.FolderID,
		row.ListID,
		row.URL,
		row.RawPayloadJSON,
		row.LastSyncedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert clickup_tasks: %w", err)
	}
	return nil
}

// InsertClickUpEvent inserts an event row. When event_id is set and duplicates
// an existing row (same event_id), returns inserted=false and the existing id.
// When event_id is NULL, each insert always creates a new row (inserted=true).
func (s *Store) InsertClickUpEvent(ctx context.Context, row ClickUpEventRow) (id uuid.UUID, inserted bool, err error) {
	if row.EventType == "" {
		return uuid.Nil, false, errors.New("event_type is required")
	}
	if len(row.PayloadJSON) == 0 {
		row.PayloadJSON = []byte("{}")
	}

	const insertQ = `
INSERT INTO clickup_events (event_id, clickup_task_id, event_type, payload)
VALUES ($1, $2, $3, $4::jsonb)
ON CONFLICT (event_id) DO NOTHING
RETURNING id
`
	err = s.db.QueryRowContext(ctx, insertQ,
		row.EventID,
		row.ClickUpTaskID,
		row.EventType,
		row.PayloadJSON,
	).Scan(&id)
	if err == nil {
		return id, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, false, fmt.Errorf("insert clickup_events: %w", err)
	}

	// ON CONFLICT DO NOTHING: no RETURNING row. Re-fetch by event_id when present.
	if row.EventID.Valid && row.EventID.String != "" {
		const selectQ = `SELECT id FROM clickup_events WHERE event_id = $1 LIMIT 1`
		err = s.db.QueryRowContext(ctx, selectQ, row.EventID.String).Scan(&id)
		if err != nil {
			return uuid.Nil, false, fmt.Errorf("select duplicate clickup_events: %w", err)
		}
		return id, false, nil
	}

	// sql.ErrNoRows with NULL event_id should not happen (INSERT always inserts).
	return uuid.Nil, false, errors.New("unexpected insert outcome for clickup_events")
}

// MarkEventProcessed sets processed=true and timestamps for the given event id.
func (s *Store) MarkEventProcessed(ctx context.Context, eventID uuid.UUID, processedAt time.Time, errorMessage sql.NullString) error {
	const q = `
UPDATE clickup_events
SET processed = true, processed_at = $2, error_message = $3
WHERE id = $1
`
	result, err := s.db.ExecContext(ctx, q, eventID, processedAt, errorMessage)
	if err != nil {
		return fmt.Errorf("mark event processed: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// CreateMilestoneGeneration inserts a generation row. The task must already exist
// in clickup_tasks (FK). status should be pending or processing.
func (s *Store) CreateMilestoneGeneration(ctx context.Context, clickUpTaskID, status, generationVersion, promptVersion, model string) (uuid.UUID, error) {
	if clickUpTaskID == "" {
		return uuid.Nil, errors.New("clickup_task_id is required")
	}
	switch status {
	case "pending", "processing":
	default:
		return uuid.Nil, fmt.Errorf("invalid initial status %q", status)
	}
	const q = `
INSERT INTO milestone_generations (
    clickup_task_id, status, generation_version, prompt_version, model, started_at
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id
`
	var id uuid.UUID
	started := time.Now().UTC()
	err := s.db.QueryRowContext(ctx, q,
		clickUpTaskID, status, generationVersion, promptVersion, model, started,
	).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("create milestone_generations: %w", err)
	}
	return id, nil
}

// MarkGenerationProcessing moves a row from pending to processing.
func (s *Store) MarkGenerationProcessing(ctx context.Context, id uuid.UUID) error {
	const q = `
UPDATE milestone_generations
SET status = 'processing', started_at = COALESCE(started_at, now())
WHERE id = $1 AND status = 'pending'
`
	result, err := s.db.ExecContext(ctx, q, id)
	if err != nil {
		return fmt.Errorf("mark generation processing: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// MarkGenerationCompleted sets terminal success fields.
func (s *Store) MarkGenerationCompleted(ctx context.Context, id uuid.UUID, fileName, storageBucket, storagePath, sha256 string, emailSentAt sql.NullTime) error {
	const q = `
UPDATE milestone_generations
SET status = 'completed',
    file_name = $2,
    storage_bucket = $3,
    storage_path = $4,
    markdown_sha256 = $5,
    email_sent_at = $6,
    completed_at = now(),
    error_message = NULL
WHERE id = $1
`
	result, err := s.db.ExecContext(ctx, q, id, fileName, storageBucket, storagePath, sha256, emailSentAt)
	if err != nil {
		return fmt.Errorf("mark generation completed: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// MarkGenerationEmailSent sets email_sent_at for a completed generation (e.g. after successful send).
func (s *Store) MarkGenerationEmailSent(ctx context.Context, id uuid.UUID, sentAt time.Time) error {
	if sentAt.IsZero() {
		sentAt = time.Now().UTC()
	}
	const q = `
UPDATE milestone_generations
SET email_sent_at = $2
WHERE id = $1 AND status = 'completed'
`
	result, err := s.db.ExecContext(ctx, q, id, sentAt)
	if err != nil {
		return fmt.Errorf("mark generation email sent: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// MarkGenerationFailed sets status failed and error message.
func (s *Store) MarkGenerationFailed(ctx context.Context, id uuid.UUID, message string) error {
	const q = `
UPDATE milestone_generations
SET status = 'failed', error_message = $2, completed_at = now()
WHERE id = $1
`
	result, err := s.db.ExecContext(ctx, q, id, message)
	if err != nil {
		return fmt.Errorf("mark generation failed: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// LatestGenerationByTaskID returns the newest generation row for a task, or sql.ErrNoRows.
func (s *Store) LatestGenerationByTaskID(ctx context.Context, clickUpTaskID string) (MilestoneGenerationRow, error) {
	if clickUpTaskID == "" {
		return MilestoneGenerationRow{}, errors.New("clickup_task_id is required")
	}
	const q = `
SELECT id, clickup_task_id, status, generation_version, prompt_version, model,
       file_name, storage_bucket, storage_path, markdown_sha256, email_sent_at,
       error_message, started_at, completed_at, created_at
FROM milestone_generations
WHERE clickup_task_id = $1
ORDER BY created_at DESC
LIMIT 1
`
	var row MilestoneGenerationRow
	err := s.db.QueryRowContext(ctx, q, clickUpTaskID).Scan(
		&row.ID,
		&row.ClickUpTaskID,
		&row.Status,
		&row.GenerationVersion,
		&row.PromptVersion,
		&row.Model,
		&row.FileName,
		&row.StorageBucket,
		&row.StoragePath,
		&row.MarkdownSHA256,
		&row.EmailSentAt,
		&row.ErrorMessage,
		&row.StartedAt,
		&row.CompletedAt,
		&row.CreatedAt,
	)
	if err != nil {
		return MilestoneGenerationRow{}, err
	}
	return row, nil
}

const pollerStateID = "default"

// GetPollerLastPolledAt returns the stored watermark for ClickUp date_updated_gt filtering.
// If the row is missing, returns (epoch, nil) so callers can treat it as first-run.
func (s *Store) GetPollerLastPolledAt(ctx context.Context) (time.Time, error) {
	const q = `SELECT last_polled_at FROM milestone_poller_state WHERE id = $1`
	var t time.Time
	err := s.db.QueryRowContext(ctx, q, pollerStateID).Scan(&t)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Unix(0, 0).UTC(), nil
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("get poller watermark: %w", err)
	}
	return t.UTC(), nil
}

// SetPollerLastPolledAt upserts the poller watermark (typically time.Now() at end of a cycle).
func (s *Store) SetPollerLastPolledAt(ctx context.Context, at time.Time) error {
	if at.IsZero() {
		at = time.Now().UTC()
	}
	const q = `
INSERT INTO milestone_poller_state (id, last_polled_at, updated_at)
VALUES ($1, $2, now())
ON CONFLICT (id) DO UPDATE SET
    last_polled_at = EXCLUDED.last_polled_at,
    updated_at = now()
`
	_, err := s.db.ExecContext(ctx, q, pollerStateID, at.UTC())
	if err != nil {
		return fmt.Errorf("set poller watermark: %w", err)
	}
	return nil
}
