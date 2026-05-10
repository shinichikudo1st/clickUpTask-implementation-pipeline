package services

import (
	"context"

	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/models"
)

// Prompt and generation version strings for audit trails and DB rows.
const (
	PromptVersionMilestoneV1 = "milestone-prompt-v1"
	GenerationVersionV1      = "1"
)

// GeneratedMilestonePlan is validated markdown plus metadata for storage and email.
type GeneratedMilestonePlan struct {
	Markdown          string
	MarkdownSHA256    string
	FileName          string
	Model             string
	PromptVersion     string
	GenerationVersion string
}

// Generator produces a milestone markdown plan from task context.
type Generator interface {
	Generate(ctx context.Context, task models.TaskContext) (*GeneratedMilestonePlan, error)
}
