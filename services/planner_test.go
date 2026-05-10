package services

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/config"
	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/db"
	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/models"
	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/services/email"
	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/services/storage"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
)

func TestSkipCompletedPolicy(t *testing.T) {
	row := db.MilestoneGenerationRow{Status: "completed"}
	if !SkipCompletedPolicy(false, nil, row) {
		t.Fatal("should skip")
	}
	if SkipCompletedPolicy(true, nil, row) {
		t.Fatal("force must not skip")
	}
	if SkipCompletedPolicy(false, errors.New("x"), row) {
		t.Fatal("err must not skip as completed")
	}
	if SkipCompletedPolicy(false, nil, db.MilestoneGenerationRow{Status: "failed"}) {
		t.Fatal("non-completed should not skip")
	}
}

func TestTaskContextToClickUpRow(t *testing.T) {
	tc := models.TaskContext{
		TaskID:      "9x",
		Name:        "N",
		Description: "d",
		Status:      "open",
		Priority:    "1",
		SpaceID:     "s",
		FolderID:    "f",
		ListID:      "l",
		URL:         "https://u",
		RawTaskJSON: []byte(`{"ok":true}`),
		Assignees:   []models.AssigneeRef{{ID: "1", Email: "a@b.c"}},
	}
	row := taskContextToClickUpRow(tc)
	if row.ClickUpTaskID != "9x" || row.Name != "N" {
		t.Fatalf("%+v", row)
	}
	if !row.AssigneeID.Valid || row.AssigneeID.String != "1" {
		t.Fatal("assignee")
	}
}

func TestTryNewPlanner_nilStore(t *testing.T) {
	p, err := TryNewPlanner(&config.Config{}, nil)
	if err != nil || p != nil {
		t.Fatalf("got %v %v", p, err)
	}
}

type fakeCU struct {
	task *models.TaskContext
	err  error
	com  []byte
}

func (f *fakeCU) GetTask(ctx context.Context, taskID string) (*models.TaskContext, error) {
	_ = ctx
	if f.err != nil {
		return nil, f.err
	}
	out := f.task
	return out, nil
}

func (f *fakeCU) GetTaskComments(ctx context.Context, taskID string) ([]byte, error) {
	_ = ctx
	_ = taskID
	return f.com, nil
}

type fakeGen struct {
	out *GeneratedMilestonePlan
	err error
}

func (f *fakeGen) Generate(ctx context.Context, task models.TaskContext) (*GeneratedMilestonePlan, error) {
	_ = ctx
	_ = task
	return f.out, f.err
}

type memBlob struct {
	uploaded map[string][]byte
}

func (m *memBlob) Upload(ctx context.Context, bucket, objectPath string, content []byte, contentType string) error {
	_ = ctx
	_ = contentType
	if m.uploaded == nil {
		m.uploaded = make(map[string][]byte)
	}
	m.uploaded[bucket+"/"+objectPath] = append([]byte(nil), content...)
	return nil
}

func (m *memBlob) Download(ctx context.Context, bucket, objectPath string) ([]byte, error) {
	return m.uploaded[bucket+"/"+objectPath], nil
}

func (m *memBlob) SignedDownloadURL(ctx context.Context, bucket, objectPath string, expiry time.Duration) (string, error) {
	_ = ctx
	_ = bucket
	_ = objectPath
	_ = expiry
	return "", storage.ErrSignedURLUnsupported
}

func TestPlanner_GenerateForTask_happyPath(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	store := db.NewStore(sqlDB)

	taskID := "task-1"
	genID := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	mock.ExpectQuery("SELECT id, clickup_task_id, status, generation_version, prompt_version, model").
		WithArgs(taskID).
		WillReturnError(sql.ErrNoRows)

	mock.ExpectExec("INSERT INTO clickup_tasks").
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectQuery("INSERT INTO milestone_generations").
		WithArgs(taskID, "pending", GenerationVersionV1, PromptVersionMilestoneV1, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(genID))

	mock.ExpectExec("UPDATE milestone_generations").
		WithArgs(genID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectExec("UPDATE milestone_generations").
		WithArgs(genID, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectExec("UPDATE milestone_generations").
		WithArgs(genID, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	cfg := &config.Config{
		LLMModel: "gpt-test",
	}
	tc := &models.TaskContext{
		TaskID: taskID, Name: "Hello", URL: "https://clickup", RawTaskJSON: []byte(`{}`),
	}
	pl := NewPlannerWithDeps(cfg, store, &fakeCU{task: tc}, &fakeGen{
		out: &GeneratedMilestonePlan{
			Markdown:          "# Title\n\n## Objective\n\nx\n\n## Recommended Approach\n\nx\n\n## Architecture\n\nx\n\n## Environment Variables\n\n- X\n\n## Phases\n\n### Phase 1 — A\n#### Tasks\n- [ ] t\n#### Milestone 1 Checkpoint\n- [ ] c\n\n## Master Checklist\n- [ ] m\n",
			MarkdownSHA256:    "sha",
			FileName:          "task-1-hello-milestone.md",
			Model:             "gpt-test",
			PromptVersion:     PromptVersionMilestoneV1,
			GenerationVersion: GenerationVersionV1,
		},
	}, &memBlob{}, email.NoopEmailService{})

	err = pl.GenerateForTask(context.Background(), taskID, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPlanner_GenerateForTask_skipCompleted(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	store := db.NewStore(sqlDB)
	taskID := "t1"
	gid := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	now := time.Now().UTC()
	mock.ExpectQuery("SELECT id, clickup_task_id, status, generation_version, prompt_version, model").
		WithArgs(taskID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "clickup_task_id", "status", "generation_version", "prompt_version", "model",
			"file_name", "storage_bucket", "storage_path", "markdown_sha256", "email_sent_at",
			"error_message", "started_at", "completed_at", "created_at",
		}).AddRow(gid, taskID, "completed", "1", PromptVersionMilestoneV1, "gpt",
			nil, nil, nil, nil, nil, nil, now, now, now))

	pl := NewPlannerWithDeps(&config.Config{}, store, &fakeCU{task: &models.TaskContext{TaskID: taskID, Name: "n"}}, &fakeGen{}, &memBlob{}, email.NoopEmailService{})
	if err := pl.GenerateForTask(context.Background(), taskID, false); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
