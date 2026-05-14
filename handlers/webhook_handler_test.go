package handlers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/config"
	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/db"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
)

func testWebhookConfig(secret string, assignee string) *config.Config {
	return &config.Config{
		Port:                 "8080",
		APISecret:            "longenough",
		ClickUpWebhookSecret: secret,
		ClickUpAssigneeID:    assignee,
	}
}

func signClickUpWebhook(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func TestClickUpWebhook_nilStore(t *testing.T) {
	t.Parallel()
	cfg := testWebhookConfig("whsec", "")
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/clickup", bytes.NewReader([]byte(`{}`)))

	ClickUpWebhookHandler(cfg, nil)(recorder, req)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: %d", recorder.Code)
	}
}

func TestClickUpWebhook_missingWebhookSecret(t *testing.T) {
	t.Parallel()
	sqlDB, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	store := db.NewStore(sqlDB)

	cfg := testWebhookConfig("", "")
	recorder := httptest.NewRecorder()
	body := []byte(`{"event":"taskCreated","webhook_id":"w","task_id":"t"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/clickup", bytes.NewReader(body))
	req.Header.Set("X-Signature", signClickUpWebhook("x", body))

	ClickUpWebhookHandler(cfg, store)(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status: %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestClickUpWebhook_missingSignature(t *testing.T) {
	t.Parallel()
	sqlDB, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	store := db.NewStore(sqlDB)
	cfg := testWebhookConfig("whsec", "")

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/clickup", bytes.NewReader([]byte(`{}`)))

	ClickUpWebhookHandler(cfg, store)(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status: %d", recorder.Code)
	}
}

func TestClickUpWebhook_badSignature(t *testing.T) {
	t.Parallel()
	sqlDB, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	store := db.NewStore(sqlDB)
	cfg := testWebhookConfig("whsec", "")

	body := []byte(`{"event":"taskCreated","webhook_id":"w","task_id":"t","history_items":[{"id":"h1"}]}`)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/clickup", bytes.NewReader(body))
	req.Header.Set("X-Signature", "deadbeef")

	ClickUpWebhookHandler(cfg, store)(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status: %d", recorder.Code)
	}
}

func TestClickUpWebhook_invalidJSON(t *testing.T) {
	t.Parallel()
	sqlDB, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	store := db.NewStore(sqlDB)
	cfg := testWebhookConfig("whsec", "")

	body := []byte(`{`)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/clickup", bytes.NewReader(body))
	req.Header.Set("X-Signature", signClickUpWebhook("whsec", body))

	ClickUpWebhookHandler(cfg, store)(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", recorder.Code)
	}
}

func TestClickUpWebhook_unsupportedEvent(t *testing.T) {
	t.Parallel()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	store := db.NewStore(sqlDB)
	cfg := testWebhookConfig("whsec", "")

	body := []byte(`{"event":"taskPriorityUpdated","webhook_id":"w","task_id":"t1","history_items":[{"id":"h1"}]}`)
	newID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")

	mock.ExpectQuery("INSERT INTO clickup_events").
		WithArgs("w:h1", sqlmock.AnyArg(), "taskPriorityUpdated", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(newID))
	mock.ExpectExec("UPDATE clickup_events").
		WithArgs(newID, sqlmock.AnyArg(), sql.NullString{String: "unsupported_event", Valid: true}).
		WillReturnResult(sqlmock.NewResult(0, 1))

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/clickup", bytes.NewReader(body))
	req.Header.Set("X-Signature", signClickUpWebhook("whsec", body))

	ClickUpWebhookHandler(cfg, store)(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status: %d", recorder.Code)
	}
	var env struct {
		Data struct {
			Accepted bool   `json:"accepted"`
			Reason   string `json:"reason"`
		} `json:"data"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if env.Data.Accepted || env.Data.Reason != "unsupported_event" {
		t.Fatalf("data: %+v", env.Data)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

type plannerSpy struct {
	mu     sync.Mutex
	taskID string
	wg     sync.WaitGroup
}

func (p *plannerSpy) GenerateForTask(ctx context.Context, taskID string, force bool) error {
	_ = ctx
	_ = force
	p.mu.Lock()
	p.taskID = taskID
	p.mu.Unlock()
	p.wg.Done()
	return nil
}

func TestClickUpWebhook_invokesPlannerOnNewEvent(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	store := db.NewStore(sqlDB)
	cfg := testWebhookConfig("whsec", "")

	body := []byte(`{"event":"taskCreated","webhook_id":"w","task_id":"t-planner","history_items":[{"id":"h1"}]}`)
	newID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")

	mock.ExpectQuery("INSERT INTO clickup_events").
		WithArgs("w:h1", sqlmock.AnyArg(), "taskCreated", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(newID))

	mock.ExpectExec("UPDATE clickup_events").
		WithArgs(newID, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	spy := &plannerSpy{}
	spy.wg.Add(1)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/clickup", bytes.NewReader(body))
	req.Header.Set("X-Signature", signClickUpWebhook("whsec", body))

	ClickUpWebhookHandler(cfg, store, spy)(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", recorder.Code, recorder.Body.String())
	}

	done := make(chan struct{})
	go func() {
		spy.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("planner not invoked")
	}

	spy.mu.Lock()
	got := spy.taskID
	spy.mu.Unlock()
	if got != "t-planner" {
		t.Fatalf("task id: %q", got)
	}
	// MarkEventProcessed runs in the webhook goroutine after GenerateForTask returns;
	// wg.Wait unblocks when GenerateForTask finishes, so wait until SQL expectations match.
	deadline := time.Now().Add(2 * time.Second)
	var metErr error
	for time.Now().Before(deadline) {
		metErr = mock.ExpectationsWereMet()
		if metErr == nil {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if metErr != nil {
		t.Fatal(metErr)
	}
}

func TestClickUpWebhook_persistsEvent(t *testing.T) {
	t.Parallel()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	store := db.NewStore(sqlDB)
	cfg := testWebhookConfig("whsec", "")

	body := []byte(`{"event":"taskCreated","webhook_id":"w","task_id":"t1","history_items":[{"id":"h1"}]}`)
	newID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")

	mock.ExpectQuery("INSERT INTO clickup_events").
		WithArgs("w:h1", sqlmock.AnyArg(), "taskCreated", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(newID))

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/clickup", bytes.NewReader(body))
	req.Header.Set("X-Signature", signClickUpWebhook("whsec", body))

	ClickUpWebhookHandler(cfg, store)(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", recorder.Code, recorder.Body.String())
	}
	var env struct {
		Data struct {
			Accepted   bool   `json:"accepted"`
			Duplicate  bool   `json:"duplicate"`
			EventRowID string `json:"event_row_id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if !env.Data.Accepted || env.Data.Duplicate || env.Data.EventRowID != newID.String() {
		t.Fatalf("data: %+v", env.Data)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestClickUpWebhook_duplicateReplay(t *testing.T) {
	t.Parallel()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	store := db.NewStore(sqlDB)
	cfg := testWebhookConfig("whsec", "")

	body := []byte(`{"event":"taskCreated","webhook_id":"w","task_id":"t1","history_items":[{"id":"h1"}]}`)
	existing := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")

	mock.ExpectQuery("INSERT INTO clickup_events").
		WithArgs("w:h1", sqlmock.AnyArg(), "taskCreated", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	mock.ExpectQuery("SELECT id FROM clickup_events").
		WithArgs("w:h1").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(existing))

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/clickup", bytes.NewReader(body))
	req.Header.Set("X-Signature", signClickUpWebhook("whsec", body))

	ClickUpWebhookHandler(cfg, store)(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status: %d", recorder.Code)
	}
	var env struct {
		Data struct {
			Accepted  bool `json:"accepted"`
			Duplicate bool `json:"duplicate"`
		} `json:"data"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if !env.Data.Accepted || !env.Data.Duplicate {
		t.Fatalf("data: %+v", env.Data)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestClickUpWebhook_taskCreatedAcceptedWhenAssigneeFilterSet(t *testing.T) {
	t.Parallel()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	store := db.NewStore(sqlDB)
	cfg := testWebhookConfig("whsec", "184")

	body := []byte(`{"event":"taskCreated","webhook_id":"w","task_id":"t1","history_items":[{"id":"h1"}]}`)
	newID := uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")

	mock.ExpectQuery("INSERT INTO clickup_events").
		WithArgs("w:h1", sqlmock.AnyArg(), "taskCreated", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(newID))

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/clickup", bytes.NewReader(body))
	req.Header.Set("X-Signature", signClickUpWebhook("whsec", body))

	ClickUpWebhookHandler(cfg, store)(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status: %d", recorder.Code)
	}
	var env struct {
		Data struct {
			Accepted bool   `json:"accepted"`
			Reason   string `json:"reason"`
		} `json:"data"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if !env.Data.Accepted || env.Data.Reason != "" {
		t.Fatalf("data: %+v", env.Data)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestClickUpWebhook_assigneeFilterRejects(t *testing.T) {
	t.Parallel()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	store := db.NewStore(sqlDB)
	cfg := testWebhookConfig("whsec", "999")

	body := []byte(`{"event":"taskAssigneeUpdated","webhook_id":"w","task_id":"t1","history_items":[{"id":"h1","field":"assignee_add","after":{"id":184}}]}`)
	newID := uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee")

	mock.ExpectQuery("INSERT INTO clickup_events").
		WithArgs("w:h1", sqlmock.AnyArg(), "taskAssigneeUpdated", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(newID))
	mock.ExpectExec("UPDATE clickup_events").
		WithArgs(newID, sqlmock.AnyArg(), sql.NullString{String: "assignee_filter", Valid: true}).
		WillReturnResult(sqlmock.NewResult(0, 1))

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/clickup", bytes.NewReader(body))
	req.Header.Set("X-Signature", signClickUpWebhook("whsec", body))

	ClickUpWebhookHandler(cfg, store)(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status: %d", recorder.Code)
	}
	var env struct {
		Data struct {
			Accepted bool   `json:"accepted"`
			Reason   string `json:"reason"`
		} `json:"data"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if env.Data.Accepted || env.Data.Reason != "assignee_filter" {
		t.Fatalf("data: %+v", env.Data)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
