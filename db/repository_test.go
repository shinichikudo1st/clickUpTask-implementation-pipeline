package db

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
)

func TestStore_UpsertClickUpTask_validation(t *testing.T) {
	t.Parallel()
	sqlDB, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	store := NewStore(sqlDB)

	if err := store.UpsertClickUpTask(context.Background(), ClickUpTaskRow{}); err == nil {
		t.Fatal("expected error for empty clickup_task_id")
	}
	if err := store.UpsertClickUpTask(context.Background(), ClickUpTaskRow{ClickUpTaskID: "t", Name: ""}); err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestStore_UpsertClickUpTask_exec(t *testing.T) {
	t.Parallel()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	store := NewStore(sqlDB)

	mock.ExpectExec("INSERT INTO clickup_tasks").
		WithArgs(
			"task-1",
			"Name",
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	row := ClickUpTaskRow{
		ClickUpTaskID:  "task-1",
		Name:           "Name",
		RawPayloadJSON: []byte(`{}`),
		LastSyncedAt:   time.Now().UTC(),
	}
	if err := store.UpsertClickUpTask(context.Background(), row); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestStore_InsertClickUpEvent_inserted(t *testing.T) {
	t.Parallel()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	store := NewStore(sqlDB)

	newID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	mock.ExpectQuery("INSERT INTO clickup_events").
		WithArgs(sql.NullString{String: "ext-1", Valid: true}, sql.NullString{}, "taskAssigned", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(newID))

	id, inserted, err := store.InsertClickUpEvent(context.Background(), ClickUpEventRow{
		EventID:     sql.NullString{String: "ext-1", Valid: true},
		EventType:   "taskAssigned",
		PayloadJSON: []byte(`{}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !inserted || id != newID {
		t.Fatalf("got id=%v inserted=%v", id, inserted)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestStore_InsertClickUpEvent_duplicate(t *testing.T) {
	t.Parallel()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	store := NewStore(sqlDB)

	existing := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	mock.ExpectQuery("INSERT INTO clickup_events").
		WithArgs(sql.NullString{String: "ext-dup", Valid: true}, sql.NullString{}, "taskAssigned", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	mock.ExpectQuery("SELECT id FROM clickup_events").
		WithArgs("ext-dup").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(existing))

	id, inserted, err := store.InsertClickUpEvent(context.Background(), ClickUpEventRow{
		EventID:     sql.NullString{String: "ext-dup", Valid: true},
		EventType:   "taskAssigned",
		PayloadJSON: []byte(`{}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if inserted || id != existing {
		t.Fatalf("got id=%v inserted=%v", id, inserted)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestStore_InsertClickUpEvent_validation(t *testing.T) {
	t.Parallel()
	sqlDB, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	store := NewStore(sqlDB)

	_, _, err = store.InsertClickUpEvent(context.Background(), ClickUpEventRow{PayloadJSON: []byte(`{}`)})
	if err == nil {
		t.Fatal("expected error without event_type")
	}
}

func TestStore_CreateMilestoneGeneration_invalidStatus(t *testing.T) {
	t.Parallel()
	sqlDB, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	store := NewStore(sqlDB)

	_, err = store.CreateMilestoneGeneration(context.Background(), "t1", "completed", "v1", "p1", "gpt")
	if err == nil {
		t.Fatal("expected error for invalid initial status")
	}
}

func TestStore_CreateMilestoneGeneration_ok(t *testing.T) {
	t.Parallel()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	store := NewStore(sqlDB)

	genID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	mock.ExpectQuery("INSERT INTO milestone_generations").
		WithArgs("t1", "pending", "v1", "p1", "gpt", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(genID))

	id, err := store.CreateMilestoneGeneration(context.Background(), "t1", "pending", "v1", "p1", "gpt")
	if err != nil {
		t.Fatal(err)
	}
	if id != genID {
		t.Fatalf("id: %v", id)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestStore_MarkEventProcessed_noRows(t *testing.T) {
	t.Parallel()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	store := NewStore(sqlDB)

	eid := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	mock.ExpectExec("UPDATE clickup_events").
		WithArgs(eid, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = store.MarkEventProcessed(context.Background(), eid, time.Now().UTC(), sql.NullString{})
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("want ErrNoRows got %v", err)
	}
}

func TestStore_MarkGenerationCompleted_noRows(t *testing.T) {
	t.Parallel()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	store := NewStore(sqlDB)

	id := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	mock.ExpectExec("UPDATE milestone_generations").
		WithArgs(id, "f.md", "b", "p", "sha", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = store.MarkGenerationCompleted(context.Background(), id, "f.md", "b", "p", "sha", sql.NullTime{})
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("want ErrNoRows got %v", err)
	}
}

func TestStore_MarkGenerationEmailSent_ok(t *testing.T) {
	t.Parallel()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	store := NewStore(sqlDB)

	id := uuid.MustParse("66666666-6666-6666-6666-666666666666")
	mock.ExpectExec("UPDATE milestone_generations").
		WithArgs(id, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = store.MarkGenerationEmailSent(context.Background(), id, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestStore_MarkGenerationProcessing_ok(t *testing.T) {
	t.Parallel()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	store := NewStore(sqlDB)

	id := uuid.MustParse("88888888-8888-8888-8888-888888888888")
	mock.ExpectExec("UPDATE milestone_generations").
		WithArgs(id).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = store.MarkGenerationProcessing(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestStore_MarkGenerationEmailSent_noRows(t *testing.T) {
	t.Parallel()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	store := NewStore(sqlDB)

	id := uuid.MustParse("77777777-7777-7777-7777-777777777777")
	mock.ExpectExec("UPDATE milestone_generations").
		WithArgs(id, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = store.MarkGenerationEmailSent(context.Background(), id, time.Now().UTC())
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("want ErrNoRows got %v", err)
	}
}

func TestStore_LatestGenerationByTaskID_emptyArg(t *testing.T) {
	t.Parallel()
	sqlDB, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	store := NewStore(sqlDB)

	_, err = store.LatestGenerationByTaskID(context.Background(), "")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestStore_pollerWatermark_getSet(t *testing.T) {
	t.Parallel()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	store := NewStore(sqlDB)

	ts := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	mock.ExpectQuery(`SELECT last_polled_at FROM milestone_poller_state WHERE id = \$1`).
		WithArgs("default").
		WillReturnRows(sqlmock.NewRows([]string{"last_polled_at"}).AddRow(ts))

	got, err := store.GetPollerLastPolledAt(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !got.Equal(ts.UTC()) {
		t.Fatalf("got %v", got)
	}

	next := time.Date(2026, 4, 2, 10, 0, 0, 0, time.UTC)
	mock.ExpectExec(`INSERT INTO milestone_poller_state`).
		WithArgs("default", next).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := store.SetPollerLastPolledAt(context.Background(), next); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
