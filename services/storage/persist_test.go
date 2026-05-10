package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/db"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
)

type errBlob struct{}

func (errBlob) Upload(ctx context.Context, bucket, objectPath string, content []byte, contentType string) error {
	_ = ctx
	_ = bucket
	_ = objectPath
	_ = content
	_ = contentType
	return errors.New("upload failed")
}

func (errBlob) Download(ctx context.Context, bucket, objectPath string) ([]byte, error) {
	return nil, errors.New("no")
}

func (errBlob) SignedDownloadURL(ctx context.Context, bucket, objectPath string, expiry time.Duration) (string, error) {
	_ = ctx
	_ = bucket
	_ = objectPath
	_ = expiry
	return "", errors.New("no")
}

func TestPersistMilestone_uploadErrorMarksFailed(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	store := db.NewStore(sqlDB)

	gid := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	mock.ExpectExec("UPDATE milestone_generations").
		WithArgs(gid, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = PersistMilestone(context.Background(), store, errBlob{}, PersistMilestoneInput{
		GenerationID:   gid,
		ClickUpTaskID:  "t1",
		PublicFileName: "f.md",
		Markdown:       []byte("# x"),
		SHA256:         "abc",
		Bucket:         "b",
	})
	if err == nil || err.Error() != "upload failed" {
		t.Fatalf("got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPersistMilestone_success(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	store := db.NewStore(sqlDB)

	gid := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	mem := &memBlob{data: map[string][]byte{}}

	mock.ExpectExec("UPDATE milestone_generations").
		WithArgs(gid, "f.md", "b", sqlmock.AnyArg(), "sha", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = PersistMilestone(context.Background(), store, mem, PersistMilestoneInput{
		GenerationID:   gid,
		ClickUpTaskID:  "t1",
		PublicFileName: "f.md",
		Markdown:       []byte("# ok"),
		SHA256:         "sha",
		Bucket:         "b",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

type memBlob struct {
	data map[string][]byte
}

func (m *memBlob) key(bucket, path string) string { return bucket + "|" + path }

func (m *memBlob) Upload(ctx context.Context, bucket, objectPath string, content []byte, contentType string) error {
	_ = ctx
	_ = contentType
	m.data[m.key(bucket, objectPath)] = append([]byte(nil), content...)
	return nil
}

func (m *memBlob) Download(ctx context.Context, bucket, objectPath string) ([]byte, error) {
	return m.data[m.key(bucket, objectPath)], nil
}

func (m *memBlob) SignedDownloadURL(ctx context.Context, bucket, objectPath string, expiry time.Duration) (string, error) {
	_ = ctx
	_ = bucket
	_ = objectPath
	_ = expiry
	return "", nil
}
