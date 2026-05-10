package email

import (
	"context"
	"fmt"
	"time"

	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/config"
	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/db"
	"github.com/google/uuid"
)

// DeliverMilestoneEmail sends the plan with retries, then records email_sent_at.
// A send failure does not change generation status (file remains stored).
func DeliverMilestoneEmail(ctx context.Context, store *db.Store, svc EmailService, cfg *config.Config, generationID uuid.UUID, in SendMilestoneInput) error {
	if store == nil {
		return fmt.Errorf("deliver: db store is nil")
	}
	if svc == nil {
		return fmt.Errorf("deliver: email service is nil")
	}
	if generationID == uuid.Nil {
		return fmt.Errorf("deliver: generation id is required")
	}
	maxAttach := 450_000
	if cfg != nil {
		maxAttach = cfg.MaxEmailAttachmentBytes()
	}
	prepared, _, err := PrepareSendInput(in, maxAttach)
	if err != nil {
		return err
	}
	if err := SendWithRetry(ctx, svc, prepared, 3); err != nil {
		return err
	}
	return store.MarkGenerationEmailSent(ctx, generationID, time.Now().UTC())
}
