package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Apex-Suite-AI/clickup-task-implementation-pipeline/models"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

func TestRecovery_ReturnsJSONEnvelopeOnPanic(t *testing.T) {
	var buf strings.Builder
	restore := captureLogOutput(&buf)
	t.Cleanup(restore)

	router := chi.NewRouter()
	router.Use(chimiddleware.RequestID)
	router.Use(Recovery)
	router.Get("/panic", func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/panic", nil))

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status: %d", recorder.Code)
	}
	var body models.ErrorResponse
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Success || body.Data != nil {
		t.Fatalf("body: %+v", body)
	}
	if body.Error.Code != "INTERNAL_ERROR" {
		t.Fatalf("code: %v", body.Error.Code)
	}
	if body.Error.Message != "internal server error" {
		t.Fatalf("message leaked? %q", body.Error.Message)
	}

	logLine := extractLogJSONLine(buf.String())
	if !strings.Contains(logLine, `"event":"panic"`) {
		t.Fatalf("expected panic in log: %s", logLine)
	}
}

func TestRecovery_SkipsBodyIfHeadersAlreadySent(t *testing.T) {
	var buf strings.Builder
	restore := captureLogOutput(&buf)
	t.Cleanup(restore)

	router := chi.NewRouter()
	router.Use(chimiddleware.RequestID)
	router.Use(Recovery)
	router.Get("/late-panic", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{"))
		panic("after write")
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/late-panic", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status: %d", recorder.Code)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "{") {
		t.Fatalf("body: %q", body)
	}
}
