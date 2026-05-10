package email

import (
	"context"
	"testing"

	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/config"
	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/db"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
)

func TestDeliverMilestoneEmail_success(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	store := db.NewStore(sqlDB)

	gid := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	mock.ExpectExec("UPDATE milestone_generations").
		WithArgs(gid, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	cfg := &config.Config{EmailMaxAttachmentBytes: 1000}
	err = DeliverMilestoneEmail(context.Background(), store, NoopEmailService{}, cfg, gid, SendMilestoneInput{
		TaskName:     "task",
		TaskURL:      "https://u",
		FileName:     "f.md",
		MarkdownBody: []byte("# x"),
		DownloadURL:  "https://dl",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDeliverMilestoneEmail_sendErrorNoDBUpdate(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	store := db.NewStore(sqlDB)

	gid := uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")
	// No SQL expectation — email fails before DB write.

	cfg := &config.Config{}
	err = DeliverMilestoneEmail(context.Background(), store, &flakySender{max: 99}, cfg, gid, SendMilestoneInput{
		TaskName:     "task",
		DownloadURL:  "https://dl",
		MarkdownBody: []byte("x"),
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDeliverMilestoneEmail_invalidInput(t *testing.T) {
	sqlDB, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	store := db.NewStore(sqlDB)
	err = DeliverMilestoneEmail(context.Background(), store, NoopEmailService{}, &config.Config{}, uuid.New(), SendMilestoneInput{
		TaskName: "",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}
