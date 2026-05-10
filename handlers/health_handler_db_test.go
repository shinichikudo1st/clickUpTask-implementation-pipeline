package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestHealthHandler_databasePingOK(t *testing.T) {
	t.Parallel()
	sqlDB, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	mock.ExpectPing()

	recorder := httptest.NewRecorder()
	HealthHandler(sqlDB)(recorder, httptest.NewRequest(http.MethodGet, "/v1/health", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status: %d", recorder.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHealthHandler_databasePingFails(t *testing.T) {
	t.Parallel()
	sqlDB, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	mock.ExpectPing().WillReturnError(errors.New("refused"))

	recorder := httptest.NewRecorder()
	HealthHandler(sqlDB)(recorder, httptest.NewRequest(http.MethodGet, "/v1/health", nil))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: %d", recorder.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHealthHandler_nilDBSkipsPing(t *testing.T) {
	t.Parallel()
	var sqlDB *sql.DB
	recorder := httptest.NewRecorder()
	HealthHandler(sqlDB)(recorder, httptest.NewRequest(http.MethodGet, "/v1/health", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status: %d", recorder.Code)
	}
}
