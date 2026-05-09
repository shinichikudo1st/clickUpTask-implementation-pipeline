package db

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// ClickUpTaskRow is a normalized task snapshot for persistence.
type ClickUpTaskRow struct {
	ClickUpTaskID  string
	Name           string
	Description    sql.NullString
	Status         sql.NullString
	Priority       sql.NullString
	AssigneeID     sql.NullString
	AssigneeEmail  sql.NullString
	SpaceID        sql.NullString
	FolderID       sql.NullString
	ListID         sql.NullString
	URL            sql.NullString
	RawPayloadJSON []byte
	LastSyncedAt   time.Time
}

// ClickUpEventRow is an incoming webhook or poller event.
type ClickUpEventRow struct {
	EventID       sql.NullString
	ClickUpTaskID sql.NullString
	EventType     string
	PayloadJSON   []byte
}

// MilestoneGenerationRow is one generation attempt.
type MilestoneGenerationRow struct {
	ID                uuid.UUID
	ClickUpTaskID     string
	Status            string
	GenerationVersion string
	PromptVersion     string
	Model             string
	FileName          sql.NullString
	StorageBucket     sql.NullString
	StoragePath       sql.NullString
	MarkdownSHA256    sql.NullString
	EmailSentAt       sql.NullTime
	ErrorMessage      sql.NullString
	StartedAt         sql.NullTime
	CompletedAt       sql.NullTime
	CreatedAt         time.Time
}
