package handlers

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
