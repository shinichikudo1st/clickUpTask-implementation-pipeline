package services

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/config"
	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/db"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
)

type pollerListStub struct {
	tasks []ListedTeamTask
	err   error
}

func (p *pollerListStub) ListTeamTasksForAssignee(ctx context.Context, teamID, assigneeID string, dateUpdatedGt time.Time) ([]ListedTeamTask, error) {
	_ = ctx
	_ = teamID
	_ = assigneeID
	_ = dateUpdatedGt
	if p.err != nil {
		return nil, p.err
	}
	return append([]ListedTeamTask(nil), p.tasks...), nil
}

type pollerRunStub struct {
	calls []string
	fail  map[string]error
}

func (p *pollerRunStub) GenerateForTask(ctx context.Context, taskID string, force bool) error {
	_ = ctx
	_ = force
	if p.fail != nil {
		if e := p.fail[taskID]; e != nil {
			return e
		}
	}
	p.calls = append(p.calls, taskID)
	return nil
}

func testPollerCfg() *config.Config {
	return &config.Config{
		APISecret:              "longenoughsecret",
		ClickUpPollerEnabled:   true,
		ClickUpTeamID:          "team1",
		ClickUpAssigneeID:      "u1",
		ClickUpPollerLookbackH: 168,
	}
}

func TestRunPollCycle_disabled(t *testing.T) {
	t.Parallel()
	cfg := testPollerCfg()
	cfg.ClickUpPollerEnabled = false
	stats, err := RunPollCycle(context.Background(), cfg, nil, &pollerListStub{}, &pollerRunStub{})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Listed != 0 || stats.Runs != 0 {
		t.Fatalf("stats=%+v", stats)
	}
}

func TestRunPollCycle_requiresTeamAndAssignee(t *testing.T) {
	t.Parallel()
	cfg := testPollerCfg()
	cfg.ClickUpTeamID = ""
	_, err := RunPollCycle(context.Background(), cfg, &db.Store{}, &pollerListStub{}, &pollerRunStub{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRunPollCycle_skipsCompletedAndRunsOthers(t *testing.T) {
	t.Parallel()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	store := db.NewStore(sqlDB)

	mock.ExpectQuery(`SELECT last_polled_at FROM milestone_poller_state WHERE id = \$1`).
		WithArgs("default").
		WillReturnRows(sqlmock.NewRows([]string{"last_polled_at"}).AddRow(time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)))

	genID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	rowsCompleted := sqlmock.NewRows([]string{
		"id", "clickup_task_id", "status", "generation_version", "prompt_version", "model",
		"file_name", "storage_bucket", "storage_path", "markdown_sha256", "email_sent_at",
		"error_message", "started_at", "completed_at", "created_at",
	}).AddRow(genID, "t2", "completed", "v1", "pv1", "m", nil, nil, nil, nil, nil, nil, time.Now(), time.Now(), time.Now())

	mock.ExpectQuery(`SELECT id, clickup_task_id, status, generation_version, prompt_version, model,
       file_name, storage_bucket, storage_path, markdown_sha256, email_sent_at,
       error_message, started_at, completed_at, created_at
FROM milestone_generations
WHERE clickup_task_id = \$1
ORDER BY created_at DESC
LIMIT 1`).
		WithArgs("t1").
		WillReturnError(sql.ErrNoRows)

	mock.ExpectQuery(`SELECT id, clickup_task_id, status, generation_version, prompt_version, model,
       file_name, storage_bucket, storage_path, markdown_sha256, email_sent_at,
       error_message, started_at, completed_at, created_at
FROM milestone_generations
WHERE clickup_task_id = \$1
ORDER BY created_at DESC
LIMIT 1`).
		WithArgs("t2").
		WillReturnRows(rowsCompleted)

	mock.ExpectExec(`INSERT INTO milestone_poller_state`).
		WillReturnResult(sqlmock.NewResult(0, 1))

	list := &pollerListStub{tasks: []ListedTeamTask{
		{ID: "t2", DateUpdated: time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)},
		{ID: "t1", DateUpdated: time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)},
	}}
	runner := &pollerRunStub{}

	stats, err := RunPollCycle(context.Background(), testPollerCfg(), store, list, runner)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Listed != 2 || stats.Runs != 1 || stats.SkippedCompleted != 1 || stats.GenerationFailures != 0 {
		t.Fatalf("stats=%+v calls=%v", stats, runner.calls)
	}
	if len(runner.calls) != 1 || runner.calls[0] != "t1" {
		t.Fatalf("calls=%v", runner.calls)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestWatermarkQueryLowerBound_firstRunUsesLookback(t *testing.T) {
	t.Parallel()
	wm := time.Unix(0, 0).UTC()
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	got := watermarkQueryLowerBound(wm, 24*time.Hour, now)
	want := now.Add(-24 * time.Hour)
	if !got.Equal(want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestWatermarkQueryLowerBound_overlap(t *testing.T) {
	t.Parallel()
	wm := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	now := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
	got := watermarkQueryLowerBound(wm, 24*time.Hour, now)
	want := wm.Add(-2 * time.Minute)
	if !got.Equal(want) {
		t.Fatalf("got %v want %v", got, want)
	}
}
