package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/config"
	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/db"
	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/services/storage"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func testCfg() *config.Config {
	return &config.Config{
		Port:                 "8080",
		APISecret:            "longsecret",
		SignedURLTTLSeconds:  3600,
	}
}

func taskRouter(cfg *config.Config, store *db.Store, planner MilestonePlanner, blobs storage.BlobStore) http.Handler {
	r := chi.NewRouter()
	RegisterTaskAPI(r, cfg, store, planner, blobs)
	return r
}

type taskPlannerSpy struct {
	mu     sync.Mutex
	taskID string
	force  bool
	wg     sync.WaitGroup
}

func (p *taskPlannerSpy) GenerateForTask(ctx context.Context, taskID string, force bool) error {
	p.mu.Lock()
	p.taskID = taskID
	p.force = force
	p.mu.Unlock()
	p.wg.Done()
	return nil
}

func TestTaskAPI_generate_unauthorized(t *testing.T) {
	t.Parallel()
	sqlDB, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	store := db.NewStore(sqlDB)
	spy := &taskPlannerSpy{}
	h := taskRouter(testCfg(), store, spy, nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/tasks/abc/generate", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestTaskAPI_generate_nilStore(t *testing.T) {
	t.Parallel()
	spy := &taskPlannerSpy{}
	h := taskRouter(testCfg(), nil, spy, nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/tasks/abc/generate", nil)
	req.Header.Set("Authorization", "Bearer longsecret")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestTaskAPI_generate_nilPlanner(t *testing.T) {
	t.Parallel()
	sqlDB, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	store := db.NewStore(sqlDB)
	h := taskRouter(testCfg(), store, nil, nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/tasks/abc/generate", nil)
	req.Header.Set("Authorization", "Bearer longsecret")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestTaskAPI_generate_asyncAndForce(t *testing.T) {
	t.Parallel()
	sqlDB, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	store := db.NewStore(sqlDB)
	spy := &taskPlannerSpy{}
	spy.wg.Add(1)
	h := taskRouter(testCfg(), store, spy, nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/tasks/t-99/generate?force=true", nil)
	req.Header.Set("Authorization", "Bearer longsecret")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	done := make(chan struct{})
	go func() {
		spy.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("planner not invoked")
	}
	spy.mu.Lock()
	defer spy.mu.Unlock()
	if spy.taskID != "t-99" || !spy.force {
		t.Fatalf("planner got taskID=%q force=%v", spy.taskID, spy.force)
	}
}

func TestTaskAPI_generate_forceFromJSONBody(t *testing.T) {
	t.Parallel()
	sqlDB, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	store := db.NewStore(sqlDB)
	spy := &taskPlannerSpy{}
	spy.wg.Add(1)
	h := taskRouter(testCfg(), store, spy, nil)

	body := []byte(`{"force":true}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/tasks/t2/generate", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer longsecret")
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status=%d", rec.Code)
	}
	spy.wg.Wait()
	spy.mu.Lock()
	if spy.taskID != "t2" || !spy.force {
		t.Fatalf("got taskID=%q force=%v", spy.taskID, spy.force)
	}
	spy.mu.Unlock()
}

func TestTaskAPI_generate_queryForceOverridesBody(t *testing.T) {
	t.Parallel()
	sqlDB, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	store := db.NewStore(sqlDB)
	spy := &taskPlannerSpy{}
	spy.wg.Add(1)
	h := taskRouter(testCfg(), store, spy, nil)

	f := false
	body, _ := json.Marshal(map[string]bool{"force": f})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/tasks/t3/generate?force=true", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer longsecret")
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rec, req)
	spy.wg.Wait()
	spy.mu.Lock()
	if !spy.force {
		t.Fatal("expected query force=true to win over JSON false")
	}
	spy.mu.Unlock()
}

func TestTaskAPI_generate_invalidJSON(t *testing.T) {
	t.Parallel()
	sqlDB, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	store := db.NewStore(sqlDB)
	spy := &taskPlannerSpy{}
	h := taskRouter(testCfg(), store, spy, nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/tasks/t4/generate", bytes.NewReader([]byte(`{`)))
	req.Header.Set("Authorization", "Bearer longsecret")
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestTaskAPI_plan_notFound(t *testing.T) {
	t.Parallel()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	store := db.NewStore(sqlDB)

	mock.ExpectQuery(`SELECT id, clickup_task_id, status, generation_version, prompt_version, model,
       file_name, storage_bucket, storage_path, markdown_sha256, email_sent_at,
       error_message, started_at, completed_at, created_at
FROM milestone_generations
WHERE clickup_task_id = \$1
ORDER BY created_at DESC
LIMIT 1`).
		WithArgs("missing").
		WillReturnError(sql.ErrNoRows)

	h := taskRouter(testCfg(), store, nil, nil)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/tasks/missing/plan", nil)
	req.Header.Set("Authorization", "Bearer longsecret")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

type fakeBlobs struct {
	url string
	err error
}

func (f *fakeBlobs) Upload(ctx context.Context, bucket, objectPath string, content []byte, contentType string) error {
	return nil
}

func (f *fakeBlobs) Download(ctx context.Context, bucket, objectPath string) ([]byte, error) {
	return nil, nil
}

func (f *fakeBlobs) SignedDownloadURL(ctx context.Context, bucket, objectPath string, expiry time.Duration) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.url, nil
}

func TestTaskAPI_plan_completedWithSignedURL(t *testing.T) {
	t.Parallel()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	store := db.NewStore(sqlDB)

	genID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	started := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	completed := started.Add(time.Minute)

	rows := sqlmock.NewRows([]string{
		"id", "clickup_task_id", "status", "generation_version", "prompt_version", "model",
		"file_name", "storage_bucket", "storage_path", "markdown_sha256", "email_sent_at",
		"error_message", "started_at", "completed_at", "created_at",
	}).AddRow(
		genID, "cu-1", "completed", "v1", "pv1", "gpt-4o-mini",
		"plan.md", "milestone-plans", "cu-1/"+genID.String()+"/plan.md", "deadbeef", nil,
		nil, started, completed, started,
	)

	mock.ExpectQuery(`SELECT id, clickup_task_id, status, generation_version, prompt_version, model,
       file_name, storage_bucket, storage_path, markdown_sha256, email_sent_at,
       error_message, started_at, completed_at, created_at
FROM milestone_generations
WHERE clickup_task_id = \$1
ORDER BY created_at DESC
LIMIT 1`).
		WithArgs("cu-1").
		WillReturnRows(rows)

	h := taskRouter(testCfg(), store, nil, &fakeBlobs{url: "https://signed.example/dl"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/tasks/cu-1/plan", nil)
	req.Header.Set("Authorization", "Bearer longsecret")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var envelope struct {
		Success bool `json:"success"`
		Data    struct {
			DownloadURL          *string `json:"download_url"`
			SignedURLAvailable   bool    `json:"signed_url_available"`
			FileAvailable        bool    `json:"file_available"`
			Generation           struct {
				Status string `json:"status"`
			} `json:"generation"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if !envelope.Success || envelope.Data.Generation.Status != "completed" {
		t.Fatalf("unexpected envelope: %+v", envelope)
	}
	if envelope.Data.DownloadURL == nil || *envelope.Data.DownloadURL != "https://signed.example/dl" {
		t.Fatalf("download_url: %+v", envelope.Data.DownloadURL)
	}
	if !envelope.Data.SignedURLAvailable || !envelope.Data.FileAvailable {
		t.Fatalf("flags: %+v", envelope.Data)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTaskAPI_plan_failedGeneration(t *testing.T) {
	t.Parallel()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	store := db.NewStore(sqlDB)

	genID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	started := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	completed := started.Add(2 * time.Minute)
	errMsg := "LLM blew up"

	rows := sqlmock.NewRows([]string{
		"id", "clickup_task_id", "status", "generation_version", "prompt_version", "model",
		"file_name", "storage_bucket", "storage_path", "markdown_sha256", "email_sent_at",
		"error_message", "started_at", "completed_at", "created_at",
	}).AddRow(
		genID, "cu-2", "failed", "v1", "pv1", "gpt-4o-mini",
		nil, nil, nil, nil, nil,
		errMsg, started, completed, started,
	)

	mock.ExpectQuery(`SELECT id, clickup_task_id, status, generation_version, prompt_version, model,
       file_name, storage_bucket, storage_path, markdown_sha256, email_sent_at,
       error_message, started_at, completed_at, created_at
FROM milestone_generations
WHERE clickup_task_id = \$1
ORDER BY created_at DESC
LIMIT 1`).
		WithArgs("cu-2").
		WillReturnRows(rows)

	h := taskRouter(testCfg(), store, nil, &fakeBlobs{url: "should-not-use"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/tasks/cu-2/plan", nil)
	req.Header.Set("Authorization", "Bearer longsecret")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	var envelope struct {
		Data struct {
			DownloadURL        interface{} `json:"download_url"`
			SignedURLAvailable bool        `json:"signed_url_available"`
			FileMessage        string      `json:"file_message"`
			Generation         struct {
				Status        string `json:"status"`
				ErrorMessage  string `json:"error_message"`
			} `json:"generation"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.Generation.Status != "failed" || envelope.Data.Generation.ErrorMessage != errMsg {
		t.Fatalf("generation: %+v", envelope.Data.Generation)
	}
	if envelope.Data.DownloadURL != nil {
		t.Fatal("expected nil download_url for failed")
	}
	if envelope.Data.SignedURLAvailable {
		t.Fatal("signed URL should not be offered for failed")
	}
}

func TestTaskAPI_plan_signedURLUnsupported(t *testing.T) {
	t.Parallel()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	store := db.NewStore(sqlDB)

	genID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	started := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	completed := started.Add(time.Minute)

	rows := sqlmock.NewRows([]string{
		"id", "clickup_task_id", "status", "generation_version", "prompt_version", "model",
		"file_name", "storage_bucket", "storage_path", "markdown_sha256", "email_sent_at",
		"error_message", "started_at", "completed_at", "created_at",
	}).AddRow(
		genID, "cu-3", "completed", "v1", "pv1", "gpt-4o-mini",
		"plan.md", "milestone-plans", "cu-3/"+genID.String()+"/plan.md", "abc", nil,
		nil, started, completed, started,
	)

	mock.ExpectQuery(`SELECT id, clickup_task_id, status, generation_version, prompt_version, model,
       file_name, storage_bucket, storage_path, markdown_sha256, email_sent_at,
       error_message, started_at, completed_at, created_at
FROM milestone_generations
WHERE clickup_task_id = \$1
ORDER BY created_at DESC
LIMIT 1`).
		WithArgs("cu-3").
		WillReturnRows(rows)

	h := taskRouter(testCfg(), store, nil, &fakeBlobs{err: storage.ErrSignedURLUnsupported})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/tasks/cu-3/plan", nil)
	req.Header.Set("Authorization", "Bearer longsecret")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"signed_url_available":false`)) {
		t.Fatalf("body=%s", rec.Body.String())
	}
}

func TestTaskAPI_plan_nilStore(t *testing.T) {
	t.Parallel()
	h := taskRouter(testCfg(), nil, nil, nil)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/tasks/x/plan", nil)
	req.Header.Set("Authorization", "Bearer longsecret")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d", rec.Code)
	}
}
