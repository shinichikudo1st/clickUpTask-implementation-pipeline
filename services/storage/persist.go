package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/db"
	"github.com/google/uuid"
)

// PersistMilestoneInput is everything needed to upload markdown and mark a generation completed.
type PersistMilestoneInput struct {
	GenerationID   uuid.UUID
	ClickUpTaskID  string
	PublicFileName string
	Markdown       []byte
	SHA256         string
	Bucket         string
}

// PersistMilestone uploads markdown to BlobStore and marks the DB row completed.
// On upload failure it marks the generation failed and returns the upload error.
func PersistMilestone(ctx context.Context, store *db.Store, blobs BlobStore, in PersistMilestoneInput) error {
	if store == nil {
		return errors.New("persist: db store is nil")
	}
	if blobs == nil {
		return errors.New("persist: blob store is nil")
	}
	if in.GenerationID == uuid.Nil {
		return errors.New("persist: generation id is required")
	}
	if len(in.Markdown) == 0 {
		msg := "persist: empty markdown"
		markFailed(ctx, store, in.GenerationID, msg)
		return errors.New(msg)
	}
	bucket := strings.TrimSpace(in.Bucket)
	if bucket == "" {
		return errors.New("persist: bucket is required")
	}
	objectPath, err := MilestoneObjectPath(in.ClickUpTaskID, in.GenerationID, in.PublicFileName)
	if err != nil {
		return err
	}

	if err := blobs.Upload(ctx, bucket, objectPath, in.Markdown, "text/markdown"); err != nil {
		markFailed(ctx, store, in.GenerationID, truncateMsg(err.Error(), 4000))
		return err
	}

	fn := strings.TrimSpace(in.PublicFileName)
	if fn == "" {
		fn = path.Base(objectPath)
	}
	if err := store.MarkGenerationCompleted(ctx, in.GenerationID, fn, bucket, objectPath, in.SHA256, sql.NullTime{}); err != nil {
		return fmt.Errorf("persist: mark completed: %w", err)
	}
	return nil
}

func markFailed(ctx context.Context, store *db.Store, id uuid.UUID, msg string) {
	if store == nil {
		return
	}
	_ = store.MarkGenerationFailed(ctx, id, msg)
}

func truncateMsg(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
