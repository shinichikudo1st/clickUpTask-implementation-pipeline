package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/config"
	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/db"
	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/models"
	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/services/email"
	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/services/storage"
)

// clickUpTaskAPI is the subset of ClickUpClient used by the planner (test doubles).
type clickUpTaskAPI interface {
	GetTask(ctx context.Context, taskID string) (*models.TaskContext, error)
	GetTaskComments(ctx context.Context, taskID string) ([]byte, error)
}

// milestoneGeneratorAPI matches Generator for tests.
type milestoneGeneratorAPI interface {
	Generate(ctx context.Context, task models.TaskContext) (*GeneratedMilestonePlan, error)
}

// Planner wires ClickUp → LLM → storage → email (Phase 8).
type Planner struct {
	cfg   *config.Config
	store *db.Store
	cu    clickUpTaskAPI
	gen   milestoneGeneratorAPI
	blobs storage.BlobStore
	mail  email.EmailService
}

// TryNewPlanner builds a Planner when all required dependencies are available.
// Returns (nil, nil) if store is nil. Returns an error if a required integration fails to construct.
func TryNewPlanner(cfg *config.Config, store *db.Store) (*Planner, error) {
	if store == nil {
		return nil, nil
	}
	if cfg == nil {
		return nil, fmt.Errorf("planner: config is nil")
	}
	cu, err := NewClickUpClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("planner: clickup: %w", err)
	}
	gen, err := NewOpenAIGenerator(cfg)
	if err != nil {
		return nil, fmt.Errorf("planner: generator: %w", err)
	}
	blobs, err := storage.NewFromConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("planner: storage: %w", err)
	}
	mail, err := email.NewFromConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("planner: email: %w", err)
	}
	return &Planner{
		cfg:   cfg,
		store: store,
		cu:    cu,
		gen:   gen,
		blobs: blobs,
		mail:  mail,
	}, nil
}

// NewPlannerWithDeps is used by tests to inject fakes.
func NewPlannerWithDeps(cfg *config.Config, store *db.Store, cu clickUpTaskAPI, gen milestoneGeneratorAPI, blobs storage.BlobStore, mail email.EmailService) *Planner {
	return &Planner{cfg: cfg, store: store, cu: cu, gen: gen, blobs: blobs, mail: mail}
}

// GenerateForTask runs the full pipeline for a ClickUp task id.
// When force is false and the latest generation for the task is already completed, returns nil without work (idempotent).
// When force is true, always creates a new generation row after fetching the task.
func (p *Planner) GenerateForTask(ctx context.Context, taskID string, force bool) error {
	if p == nil {
		return errors.New("planner is nil")
	}
	if p.store == nil {
		return errors.New("planner: store is nil")
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return errors.New("planner: task id is required")
	}

	row, errLatest := p.store.LatestGenerationByTaskID(ctx, taskID)
	if SkipCompletedPolicy(force, errLatest, row) {
		return nil
	}
	if errLatest != nil && !errors.Is(errLatest, sql.ErrNoRows) {
		return fmt.Errorf("planner: latest generation: %w", errLatest)
	}

	tc, err := p.cu.GetTask(ctx, taskID)
	if err != nil {
		return fmt.Errorf("planner: get task: %w", err)
	}

	if err := p.store.UpsertClickUpTask(ctx, taskContextToClickUpRow(*tc)); err != nil {
		return fmt.Errorf("planner: upsert task: %w", err)
	}

	modelRow := strings.TrimSpace(p.cfg.LLMModel)
	if modelRow == "" {
		modelRow = "gpt-4o-mini"
	}

	genID, err := p.store.CreateMilestoneGeneration(ctx, taskID, "pending", GenerationVersionV1, PromptVersionMilestoneV1, modelRow)
	if err != nil {
		return fmt.Errorf("planner: create generation: %w", err)
	}

	if err := p.store.MarkGenerationProcessing(ctx, genID); err != nil {
		_ = p.store.MarkGenerationFailed(ctx, genID, truncatePlannerMsg("processing transition: "+err.Error(), 4000))
		return fmt.Errorf("planner: mark processing: %w", err)
	}

	if comments, err := p.cu.GetTaskComments(ctx, taskID); err == nil && len(comments) > 0 {
		tc.CommentsJSON = comments
	}

	plan, err := p.gen.Generate(ctx, *tc)
	if err != nil {
		_ = p.store.MarkGenerationFailed(ctx, genID, truncatePlannerMsg("generation: "+err.Error(), 4000))
		return fmt.Errorf("planner: generate: %w", err)
	}

	bucket := storage.MilestoneBucketName(p.cfg)
	in := storage.PersistMilestoneInput{
		GenerationID:   genID,
		ClickUpTaskID:  taskID,
		PublicFileName: plan.FileName,
		Markdown:       []byte(plan.Markdown),
		SHA256:         plan.MarkdownSHA256,
		Bucket:         bucket,
	}
	if err := storage.PersistMilestone(ctx, p.store, p.blobs, in); err != nil {
		return fmt.Errorf("planner: persist: %w", err)
	}

	objPath, err := storage.MilestoneObjectPath(taskID, genID, plan.FileName)
	if err != nil {
		return fmt.Errorf("planner: object path: %w", err)
	}
	downloadURL := ""
	if ttl := p.cfg.SignedURLTTL(); ttl > 0 {
		u, err := p.blobs.SignedDownloadURL(ctx, bucket, objPath, ttl)
		if err == nil {
			downloadURL = u
		} else if !errors.Is(err, storage.ErrSignedURLUnsupported) {
			return fmt.Errorf("planner: signed url: %w", err)
		}
	}

	mailIn := email.SendMilestoneInput{
		TaskName:       tc.Name,
		TaskURL:        tc.URL,
		FileName:       plan.FileName,
		MarkdownBody:   []byte(plan.Markdown),
		DownloadURL:    downloadURL,
	}
	if err := email.DeliverMilestoneEmail(ctx, p.store, p.mail, p.cfg, genID, mailIn); err != nil {
		// File is already stored; do not fail the whole run (Phase 7 semantics).
		return fmt.Errorf("planner: email: %w", err)
	}
	return nil
}

func taskContextToClickUpRow(tc models.TaskContext) db.ClickUpTaskRow {
	raw := append([]byte(nil), tc.RawTaskJSON...)
	if len(raw) == 0 {
		raw = []byte("{}")
	}
	row := db.ClickUpTaskRow{
		ClickUpTaskID:  tc.TaskID,
		Name:           tc.Name,
		Description:    sql.NullString{String: tc.Description, Valid: tc.Description != ""},
		Status:         sql.NullString{String: tc.Status, Valid: tc.Status != ""},
		Priority:       sql.NullString{String: tc.Priority, Valid: tc.Priority != ""},
		SpaceID:        sql.NullString{String: tc.SpaceID, Valid: tc.SpaceID != ""},
		FolderID:       sql.NullString{String: tc.FolderID, Valid: tc.FolderID != ""},
		ListID:         sql.NullString{String: tc.ListID, Valid: tc.ListID != ""},
		URL:            sql.NullString{String: tc.URL, Valid: tc.URL != ""},
		RawPayloadJSON: raw,
		LastSyncedAt:   time.Now().UTC(),
	}
	if len(tc.Assignees) > 0 {
		row.AssigneeID = sql.NullString{String: tc.Assignees[0].ID, Valid: tc.Assignees[0].ID != ""}
		row.AssigneeEmail = sql.NullString{String: tc.Assignees[0].Email, Valid: tc.Assignees[0].Email != ""}
	}
	return row
}

func truncatePlannerMsg(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
